// scene: a 3D scene Go describes (plugins/scene), drawn with Vulkan.
//
// The first half draws into any set of image views: it knows nothing of
// Android, so it runs offscreen in tests. The second half, for Android
// only, makes a swapchain on a Surface and is what Scene.kt calls.
//
// Each frame: if shadows are on, a depth-only pass from the first light
// into the shadow map; then the scene, lit, textured, and shadowed by
// sampling that map around each point. Each node's Spin turns here.

#include <math.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <vulkan/vulkan.h>

#include "scene_spv.h"

#define SC_FRAMES 2   // frames in flight
#define SC_LIGHTS 4
#define SC_SHADOW 2048 // the shadow map's side

typedef struct {
    char key[32];
    VkBuffer vb, ib;
    VkDeviceMemory vm, im;
    uint32_t count;
    int used;
} sc_mesh;

typedef struct {
    char key[32];
    VkImage image;
    VkDeviceMemory mem;
    VkImageView view;
    VkDescriptorPool pool;
    VkDescriptorSet set;
    int used;
} sc_texture;

typedef struct {
    int mesh, texture; // into meshes and textures, or -1
    float base[16], local[16];
    float color[4];
    float axis[3], speed, shine, unlit, twosided, cast;
    int parent;        // 1 + the index of the spinning node it turns with, or 0
    float world[16];   // where it is this frame, from sc_place
    double started;    // when it began to spin
    char id[64];
} sc_node;

typedef struct { // matches the shaders' Scene block (std140)
    float vp[16], light_vp[16];
    float eye[4], ambient[4];
    float dirs[SC_LIGHTS][4], colors[SC_LIGHTS][4];
    int32_t count[4];
} sc_ubo;

typedef struct { // matches the shaders' push constants
    float model[16];
    float color[4];
    float params[4];
} sc_push;

typedef struct {
    VkPhysicalDevice pd;
    VkDevice dev;
    VkQueue queue;
    uint32_t family;
    VkFormat color_fmt, depth_fmt;
    VkRenderPass pass, shadow_pass;
    VkDescriptorSetLayout dsl, tex_dsl;
    VkPipelineLayout layout;
    VkPipeline pipeline, pipeline_twosided, shadow_pipeline;
    VkDescriptorPool pool;
    VkDescriptorSet sets[SC_FRAMES];
    VkBuffer ubo[SC_FRAMES];
    VkDeviceMemory ubo_mem[SC_FRAMES];
    void *ubo_map[SC_FRAMES];
    VkCommandPool cmd_pool;
    VkCommandBuffer cmds[SC_FRAMES];
    VkFence fences[SC_FRAMES];
    uint32_t frame;
    VkSampler sampler, shadow_sampler;
    sc_texture white; // for nodes without a picture

    // The shadow map.
    VkImage shadow;
    VkDeviceMemory shadow_mem;
    VkImageView shadow_view;
    VkFramebuffer shadow_fb;

    // The target: images to draw into, with multisampled color and depth.
    uint32_t width, height, nviews;
    VkImageView *views;
    VkFramebuffer *fbs;
    VkImage msaa, depth;
    VkDeviceMemory msaa_mem, depth_mem;
    VkImageView msaa_view, depth_view;

    // The scene.
    float clear[4], eye[3], target[3], fov, near_, far_, ambient;
    int shadows;
    float light_vp[16];
    int nlights;
    float light_dir[SC_LIGHTS][3], light_col[SC_LIGHTS][3];
    sc_mesh *meshes;
    int nmeshes;
    sc_texture *textures;
    int ntextures;
    sc_node *nodes;
    int nnodes;
} sc;

// ---- Matrices, column by column.

static void mat_mul(float *out, const float *a, const float *b) {
    float m[16];
    for (int c = 0; c < 4; c++)
        for (int r = 0; r < 4; r++) {
            float s = 0;
            for (int k = 0; k < 4; k++) s += a[k * 4 + r] * b[c * 4 + k];
            m[c * 4 + r] = s;
        }
    memcpy(out, m, sizeof m);
}

static void mat_axis_angle(float *m, const float *axis, float rad) {
    float x = axis[0], y = axis[1], z = axis[2];
    float l = sqrtf(x * x + y * y + z * z);
    memset(m, 0, 16 * sizeof(float));
    m[15] = 1;
    if (l == 0) { m[0] = m[5] = m[10] = 1; return; }
    x /= l; y /= l; z /= l;
    float c = cosf(rad), s = sinf(rad), t = 1 - c;
    m[0] = t * x * x + c;     m[1] = t * x * y + s * z; m[2] = t * x * z - s * y;
    m[4] = t * x * y - s * z; m[5] = t * y * y + c;     m[6] = t * y * z + s * x;
    m[8] = t * x * z + s * y; m[9] = t * y * z - s * x; m[10] = t * z * z + c;
}

// Vulkan's clip space: y down, depth 0 to 1.
static void mat_projection(float *m, float fov_deg, float aspect, float n, float f) {
    float ys = 1 / tanf(fov_deg * (float)M_PI / 360), xs = ys / aspect, zs = f / (n - f);
    memset(m, 0, 16 * sizeof(float));
    m[0] = xs; m[5] = -ys; m[10] = zs; m[11] = -1; m[14] = zs * n;
}

static void mat_look_at(float *m, const float *eye, const float *target) {
    float f[3] = {target[0] - eye[0], target[1] - eye[1], target[2] - eye[2]};
    float l = sqrtf(f[0] * f[0] + f[1] * f[1] + f[2] * f[2]);
    if (l == 0) { f[2] = -1; l = 1; }
    for (int i = 0; i < 3; i++) f[i] /= l;
    float up[3] = {0, 1, 0};
    if (fabsf(f[1]) > 0.999f) { up[1] = 0; up[2] = -1; }
    float s[3] = {f[1] * up[2] - f[2] * up[1], f[2] * up[0] - f[0] * up[2], f[0] * up[1] - f[1] * up[0]};
    l = sqrtf(s[0] * s[0] + s[1] * s[1] + s[2] * s[2]);
    for (int i = 0; i < 3; i++) s[i] /= l;
    float u[3] = {s[1] * f[2] - s[2] * f[1], s[2] * f[0] - s[0] * f[2], s[0] * f[1] - s[1] * f[0]};
    float out[16] = {s[0], u[0], -f[0], 0, s[1], u[1], -f[1], 0, s[2], u[2], -f[2], 0,
        -(s[0] * eye[0] + s[1] * eye[1] + s[2] * eye[2]), -(u[0] * eye[0] + u[1] * eye[1] + u[2] * eye[2]),
        f[0] * eye[0] + f[1] * eye[1] + f[2] * eye[2], 1};
    memcpy(m, out, sizeof out);
}

// ---- Memory.

static int sc_memory_type(sc *s, uint32_t bits, VkMemoryPropertyFlags want) {
    VkPhysicalDeviceMemoryProperties p;
    vkGetPhysicalDeviceMemoryProperties(s->pd, &p);
    for (uint32_t i = 0; i < p.memoryTypeCount; i++)
        if ((bits & (1u << i)) && (p.memoryTypes[i].propertyFlags & want) == want) return (int)i;
    return -1;
}

static VkResult sc_buffer(sc *s, VkDeviceSize size, VkBufferUsageFlags usage, const void *data, VkBuffer *b, VkDeviceMemory *m, void **mapped) {
    VkBufferCreateInfo bi = {VK_STRUCTURE_TYPE_BUFFER_CREATE_INFO, .size = size, .usage = usage, .sharingMode = VK_SHARING_MODE_EXCLUSIVE};
    VkResult r = vkCreateBuffer(s->dev, &bi, NULL, b);
    if (r != VK_SUCCESS) return r;
    VkMemoryRequirements req;
    vkGetBufferMemoryRequirements(s->dev, *b, &req);
    int type = sc_memory_type(s, req.memoryTypeBits, VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT);
    if (type < 0) return VK_ERROR_OUT_OF_DEVICE_MEMORY;
    VkMemoryAllocateInfo ai = {VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO, .allocationSize = req.size, .memoryTypeIndex = (uint32_t)type};
    if ((r = vkAllocateMemory(s->dev, &ai, NULL, m)) != VK_SUCCESS) return r;
    vkBindBufferMemory(s->dev, *b, *m, 0);
    void *p;
    if ((r = vkMapMemory(s->dev, *m, 0, size, 0, &p)) != VK_SUCCESS) return r;
    if (data) memcpy(p, data, size);
    if (mapped) *mapped = p; else vkUnmapMemory(s->dev, *m);
    return VK_SUCCESS;
}

static VkResult sc_image2(sc *s, uint32_t w, uint32_t h, uint32_t mips, VkSampleCountFlagBits samples, VkFormat fmt,
                          VkImageUsageFlags usage, VkImageAspectFlags aspect, int lazy, VkImage *img, VkDeviceMemory *mem, VkImageView *view) {
    VkImageCreateInfo ii = {VK_STRUCTURE_TYPE_IMAGE_CREATE_INFO, .imageType = VK_IMAGE_TYPE_2D, .format = fmt,
        .extent = {w, h, 1}, .mipLevels = mips, .arrayLayers = 1, .samples = samples,
        .tiling = VK_IMAGE_TILING_OPTIMAL, .usage = usage, .sharingMode = VK_SHARING_MODE_EXCLUSIVE, .initialLayout = VK_IMAGE_LAYOUT_UNDEFINED};
    VkResult r = vkCreateImage(s->dev, &ii, NULL, img);
    if (r != VK_SUCCESS) return r;
    VkMemoryRequirements req;
    vkGetImageMemoryRequirements(s->dev, *img, &req);
    // Lazily allocated where the GPU has it: tiled GPUs keep these on chip.
    int type = lazy ? sc_memory_type(s, req.memoryTypeBits, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT | VK_MEMORY_PROPERTY_LAZILY_ALLOCATED_BIT) : -1;
    if (type < 0) type = sc_memory_type(s, req.memoryTypeBits, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT);
    if (type < 0) return VK_ERROR_OUT_OF_DEVICE_MEMORY;
    VkMemoryAllocateInfo ai = {VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO, .allocationSize = req.size, .memoryTypeIndex = (uint32_t)type};
    if ((r = vkAllocateMemory(s->dev, &ai, NULL, mem)) != VK_SUCCESS) return r;
    vkBindImageMemory(s->dev, *img, *mem, 0);
    VkImageViewCreateInfo vi = {VK_STRUCTURE_TYPE_IMAGE_VIEW_CREATE_INFO, .image = *img, .viewType = VK_IMAGE_VIEW_TYPE_2D, .format = fmt,
        .subresourceRange = {aspect, 0, mips, 0, 1}};
    return vkCreateImageView(s->dev, &vi, NULL, view);
}

// sc_once records commands with f, runs them, and waits.
static VkResult sc_once(sc *s, void (*f)(VkCommandBuffer, void *), void *arg) {
    VkCommandBufferAllocateInfo ca = {VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO, .commandPool = s->cmd_pool, .level = VK_COMMAND_BUFFER_LEVEL_PRIMARY, .commandBufferCount = 1};
    VkCommandBuffer cb;
    VkResult r = vkAllocateCommandBuffers(s->dev, &ca, &cb);
    if (r != VK_SUCCESS) return r;
    VkCommandBufferBeginInfo bi = {VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO, .flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT};
    vkBeginCommandBuffer(cb, &bi);
    f(cb, arg);
    vkEndCommandBuffer(cb);
    VkFenceCreateInfo fi = {VK_STRUCTURE_TYPE_FENCE_CREATE_INFO};
    VkFence fence;
    vkCreateFence(s->dev, &fi, NULL, &fence);
    VkSubmitInfo si = {VK_STRUCTURE_TYPE_SUBMIT_INFO, .commandBufferCount = 1, .pCommandBuffers = &cb};
    r = vkQueueSubmit(s->queue, 1, &si, fence);
    if (r == VK_SUCCESS) vkWaitForFences(s->dev, 1, &fence, VK_TRUE, UINT64_MAX);
    vkDestroyFence(s->dev, fence, NULL);
    vkFreeCommandBuffers(s->dev, s->cmd_pool, 1, &cb);
    return r;
}

static void sc_barrier(VkCommandBuffer cb, VkImage img, uint32_t mip, VkImageLayout from, VkImageLayout to,
                       VkAccessFlags src_access, VkAccessFlags dst_access, VkPipelineStageFlags src, VkPipelineStageFlags dst) {
    VkImageMemoryBarrier b = {VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER, .srcAccessMask = src_access, .dstAccessMask = dst_access,
        .oldLayout = from, .newLayout = to, .srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED, .dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED,
        .image = img, .subresourceRange = {VK_IMAGE_ASPECT_COLOR_BIT, mip, 1, 0, 1}};
    vkCmdPipelineBarrier(cb, src, dst, 0, 0, NULL, 0, NULL, 1, &b);
}

typedef struct { VkBuffer staging; VkImage image; uint32_t w, h, mips; } sc_upload;

// Copies the picture in, then halves it down to 1×1, each level from the
// one above, with linear filtering.
static void sc_record_upload(VkCommandBuffer cb, void *arg) {
    sc_upload *u = arg;
    sc_barrier(cb, u->image, 0, VK_IMAGE_LAYOUT_UNDEFINED, VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL, 0, VK_ACCESS_TRANSFER_WRITE_BIT,
               VK_PIPELINE_STAGE_TOP_OF_PIPE_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT);
    VkBufferImageCopy copy = {0, 0, 0, {VK_IMAGE_ASPECT_COLOR_BIT, 0, 0, 1}, {0, 0, 0}, {u->w, u->h, 1}};
    vkCmdCopyBufferToImage(cb, u->staging, u->image, VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL, 1, &copy);
    int32_t w = (int32_t)u->w, h = (int32_t)u->h;
    for (uint32_t m = 1; m < u->mips; m++) {
        sc_barrier(cb, u->image, m - 1, VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL, VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL,
                   VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT);
        sc_barrier(cb, u->image, m, VK_IMAGE_LAYOUT_UNDEFINED, VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL, 0, VK_ACCESS_TRANSFER_WRITE_BIT,
                   VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT);
        int32_t nw = w > 1 ? w / 2 : 1, nh = h > 1 ? h / 2 : 1;
        VkImageBlit blit = {{VK_IMAGE_ASPECT_COLOR_BIT, m - 1, 0, 1}, {{0, 0, 0}, {w, h, 1}},
                            {VK_IMAGE_ASPECT_COLOR_BIT, m, 0, 1}, {{0, 0, 0}, {nw, nh, 1}}};
        vkCmdBlitImage(cb, u->image, VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL, u->image, VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL, 1, &blit, VK_FILTER_LINEAR);
        sc_barrier(cb, u->image, m - 1, VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL, VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL,
                   VK_ACCESS_TRANSFER_READ_BIT, VK_ACCESS_SHADER_READ_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_FRAGMENT_SHADER_BIT);
        w = nw; h = nh;
    }
    sc_barrier(cb, u->image, u->mips - 1, VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL, VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL,
               VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_FRAGMENT_SHADER_BIT);
}

// sc_make_texture uploads RGBA pixels, w×h, rows from the top.
static VkResult sc_make_texture(sc *s, sc_texture *t, const void *rgba, uint32_t w, uint32_t h) {
    uint32_t mips = 1;
    for (uint32_t m = w > h ? w : h; m > 1; m >>= 1) mips++;
    VkResult r = sc_image2(s, w, h, mips, VK_SAMPLE_COUNT_1_BIT, VK_FORMAT_R8G8B8A8_UNORM,
                           VK_IMAGE_USAGE_SAMPLED_BIT | VK_IMAGE_USAGE_TRANSFER_DST_BIT | VK_IMAGE_USAGE_TRANSFER_SRC_BIT,
                           VK_IMAGE_ASPECT_COLOR_BIT, 0, &t->image, &t->mem, &t->view);
    if (r != VK_SUCCESS) return r;
    VkBuffer staging;
    VkDeviceMemory smem;
    if ((r = sc_buffer(s, (VkDeviceSize)w * h * 4, VK_BUFFER_USAGE_TRANSFER_SRC_BIT, rgba, &staging, &smem, NULL)) != VK_SUCCESS) return r;
    sc_upload u = {staging, t->image, w, h, mips};
    r = sc_once(s, sc_record_upload, &u);
    vkDestroyBuffer(s->dev, staging, NULL);
    vkFreeMemory(s->dev, smem, NULL);
    if (r != VK_SUCCESS) return r;
    VkDescriptorPoolSize ps = {VK_DESCRIPTOR_TYPE_COMBINED_IMAGE_SAMPLER, 1};
    VkDescriptorPoolCreateInfo dp = {VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO, .maxSets = 1, .poolSizeCount = 1, .pPoolSizes = &ps};
    if ((r = vkCreateDescriptorPool(s->dev, &dp, NULL, &t->pool)) != VK_SUCCESS) return r;
    VkDescriptorSetAllocateInfo da = {VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO, .descriptorPool = t->pool, .descriptorSetCount = 1, .pSetLayouts = &s->tex_dsl};
    if ((r = vkAllocateDescriptorSets(s->dev, &da, &t->set)) != VK_SUCCESS) return r;
    VkDescriptorImageInfo ii = {s->sampler, t->view, VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL};
    VkWriteDescriptorSet wr = {VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET, .dstSet = t->set, .dstBinding = 0, .descriptorCount = 1,
        .descriptorType = VK_DESCRIPTOR_TYPE_COMBINED_IMAGE_SAMPLER, .pImageInfo = &ii};
    vkUpdateDescriptorSets(s->dev, 1, &wr, 0, NULL);
    return VK_SUCCESS;
}

static void sc_free_texture(sc *s, sc_texture *t) {
    if (t->pool) vkDestroyDescriptorPool(s->dev, t->pool, NULL);
    if (t->view) vkDestroyImageView(s->dev, t->view, NULL);
    if (t->image) vkDestroyImage(s->dev, t->image, NULL);
    if (t->mem) vkFreeMemory(s->dev, t->mem, NULL);
    memset(t, 0, sizeof *t);
}

// ---- Setup.

static VkShaderModule sc_shader(sc *s, const uint32_t *code, size_t size) {
    VkShaderModuleCreateInfo ci = {VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO, .codeSize = size, .pCode = code};
    VkShaderModule m = VK_NULL_HANDLE;
    vkCreateShaderModule(s->dev, &ci, NULL, &m);
    return m;
}

static VkFormat sc_depth_format(VkPhysicalDevice pd) {
    VkFormat list[] = {VK_FORMAT_D32_SFLOAT, VK_FORMAT_D24_UNORM_S8_UINT, VK_FORMAT_D16_UNORM};
    for (int i = 0; i < 3; i++) {
        VkFormatProperties p;
        vkGetPhysicalDeviceFormatProperties(pd, list[i], &p);
        if (p.optimalTilingFeatures & VK_FORMAT_FEATURE_DEPTH_STENCIL_ATTACHMENT_BIT) return list[i];
    }
    return VK_FORMAT_D16_UNORM;
}

// The pipelines: the scene's, culling back faces or not, and the shadow
// pass's, depth only, nudged back so a surface doesn't shadow itself.
static VkResult sc_pipeline(sc *s, int shadow, VkCullModeFlags cull, VkPipeline *out) {
    VkShaderModule vert = shadow ? sc_shader(s, shadow_vert, sizeof shadow_vert) : sc_shader(s, scene_vert, sizeof scene_vert);
    VkShaderModule frag = shadow ? VK_NULL_HANDLE : sc_shader(s, scene_frag, sizeof scene_frag);
    VkPipelineShaderStageCreateInfo stages[2] = {
        {VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO, .stage = VK_SHADER_STAGE_VERTEX_BIT, .module = vert, .pName = "main"},
        {VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO, .stage = VK_SHADER_STAGE_FRAGMENT_BIT, .module = frag, .pName = "main"},
    };
    VkVertexInputBindingDescription vb = {0, 32, VK_VERTEX_INPUT_RATE_VERTEX};
    VkVertexInputAttributeDescription va[3] = {{0, 0, VK_FORMAT_R32G32B32_SFLOAT, 0}, {1, 0, VK_FORMAT_R32G32B32_SFLOAT, 12}, {2, 0, VK_FORMAT_R32G32_SFLOAT, 24}};
    VkPipelineVertexInputStateCreateInfo vi = {VK_STRUCTURE_TYPE_PIPELINE_VERTEX_INPUT_STATE_CREATE_INFO,
        .vertexBindingDescriptionCount = 1, .pVertexBindingDescriptions = &vb, .vertexAttributeDescriptionCount = shadow ? 1 : 3, .pVertexAttributeDescriptions = va};
    VkPipelineInputAssemblyStateCreateInfo ia = {VK_STRUCTURE_TYPE_PIPELINE_INPUT_ASSEMBLY_STATE_CREATE_INFO, .topology = VK_PRIMITIVE_TOPOLOGY_TRIANGLE_LIST};
    VkPipelineViewportStateCreateInfo vp = {VK_STRUCTURE_TYPE_PIPELINE_VIEWPORT_STATE_CREATE_INFO, .viewportCount = 1, .scissorCount = 1};
    // Go's front faces wind counter-clockwise. The projection's flipped y
    // and Vulkan's y-down framebuffer cancel out, so they still do here
    // (checked by drawing on lavapipe).
    VkPipelineRasterizationStateCreateInfo rs = {VK_STRUCTURE_TYPE_PIPELINE_RASTERIZATION_STATE_CREATE_INFO,
        .polygonMode = VK_POLYGON_MODE_FILL, .cullMode = cull, .frontFace = VK_FRONT_FACE_COUNTER_CLOCKWISE, .lineWidth = 1,
        .depthBiasEnable = shadow ? VK_TRUE : VK_FALSE, .depthBiasConstantFactor = 1.25f, .depthBiasSlopeFactor = 1.75f};
    VkPipelineMultisampleStateCreateInfo ms = {VK_STRUCTURE_TYPE_PIPELINE_MULTISAMPLE_STATE_CREATE_INFO,
        .rasterizationSamples = shadow ? VK_SAMPLE_COUNT_1_BIT : VK_SAMPLE_COUNT_4_BIT};
    VkPipelineDepthStencilStateCreateInfo ds = {VK_STRUCTURE_TYPE_PIPELINE_DEPTH_STENCIL_STATE_CREATE_INFO,
        .depthTestEnable = VK_TRUE, .depthWriteEnable = VK_TRUE, .depthCompareOp = VK_COMPARE_OP_LESS_OR_EQUAL};
    VkPipelineColorBlendAttachmentState blend = {VK_TRUE, VK_BLEND_FACTOR_SRC_ALPHA, VK_BLEND_FACTOR_ONE_MINUS_SRC_ALPHA, VK_BLEND_OP_ADD,
        VK_BLEND_FACTOR_ONE, VK_BLEND_FACTOR_ONE_MINUS_SRC_ALPHA, VK_BLEND_OP_ADD, 0xf};
    VkPipelineColorBlendStateCreateInfo cb = {VK_STRUCTURE_TYPE_PIPELINE_COLOR_BLEND_STATE_CREATE_INFO, .attachmentCount = shadow ? 0 : 1, .pAttachments = &blend};
    VkDynamicState dyn[2] = {VK_DYNAMIC_STATE_VIEWPORT, VK_DYNAMIC_STATE_SCISSOR};
    VkPipelineDynamicStateCreateInfo dy = {VK_STRUCTURE_TYPE_PIPELINE_DYNAMIC_STATE_CREATE_INFO, .dynamicStateCount = 2, .pDynamicStates = dyn};
    VkGraphicsPipelineCreateInfo gp = {VK_STRUCTURE_TYPE_GRAPHICS_PIPELINE_CREATE_INFO, .stageCount = shadow ? 1 : 2, .pStages = stages,
        .pVertexInputState = &vi, .pInputAssemblyState = &ia, .pViewportState = &vp, .pRasterizationState = &rs,
        .pMultisampleState = &ms, .pDepthStencilState = &ds, .pColorBlendState = &cb, .pDynamicState = &dy,
        .layout = s->layout, .renderPass = shadow ? s->shadow_pass : s->pass};
    VkResult r = vkCreateGraphicsPipelines(s->dev, VK_NULL_HANDLE, 1, &gp, NULL, out);
    vkDestroyShaderModule(s->dev, vert, NULL);
    if (frag) vkDestroyShaderModule(s->dev, frag, NULL);
    return r;
}

static void sc_free(sc *s);

// sc_new sets up the drawing for images of color_fmt, on a device and queue
// the caller made. final_layout is what the images are left in: present
// for a swapchain, transfer source for a test.
static sc *sc_new(VkPhysicalDevice pd, VkDevice dev, uint32_t family, VkQueue queue, VkFormat color_fmt, VkImageLayout final_layout) {
    sc *s = calloc(1, sizeof *s);
    s->pd = pd; s->dev = dev; s->family = family; s->queue = queue; s->color_fmt = color_fmt; s->depth_fmt = sc_depth_format(pd);
    s->fov = 60; s->near_ = 0.1f; s->far_ = 100; s->ambient = 0.25f;
    s->eye[2] = 5;

    VkAttachmentDescription att[3] = {
        {0, color_fmt, VK_SAMPLE_COUNT_4_BIT, VK_ATTACHMENT_LOAD_OP_CLEAR, VK_ATTACHMENT_STORE_OP_DONT_CARE,
         VK_ATTACHMENT_LOAD_OP_DONT_CARE, VK_ATTACHMENT_STORE_OP_DONT_CARE, VK_IMAGE_LAYOUT_UNDEFINED, VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL},
        {0, s->depth_fmt, VK_SAMPLE_COUNT_4_BIT, VK_ATTACHMENT_LOAD_OP_CLEAR, VK_ATTACHMENT_STORE_OP_DONT_CARE,
         VK_ATTACHMENT_LOAD_OP_DONT_CARE, VK_ATTACHMENT_STORE_OP_DONT_CARE, VK_IMAGE_LAYOUT_UNDEFINED, VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL},
        {0, color_fmt, VK_SAMPLE_COUNT_1_BIT, VK_ATTACHMENT_LOAD_OP_DONT_CARE, VK_ATTACHMENT_STORE_OP_STORE,
         VK_ATTACHMENT_LOAD_OP_DONT_CARE, VK_ATTACHMENT_STORE_OP_DONT_CARE, VK_IMAGE_LAYOUT_UNDEFINED, final_layout},
    };
    VkAttachmentReference color = {0, VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL};
    VkAttachmentReference depth = {1, VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL};
    VkAttachmentReference resolve = {2, VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL};
    VkSubpassDescription sub = {0, VK_PIPELINE_BIND_POINT_GRAPHICS, .colorAttachmentCount = 1, .pColorAttachments = &color,
        .pResolveAttachments = &resolve, .pDepthStencilAttachment = &depth};
    VkSubpassDependency dep = {VK_SUBPASS_EXTERNAL, 0,
        VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT | VK_PIPELINE_STAGE_EARLY_FRAGMENT_TESTS_BIT,
        VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT | VK_PIPELINE_STAGE_EARLY_FRAGMENT_TESTS_BIT,
        0, VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT | VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_WRITE_BIT, 0};
    VkRenderPassCreateInfo rp = {VK_STRUCTURE_TYPE_RENDER_PASS_CREATE_INFO, .attachmentCount = 3, .pAttachments = att,
        .subpassCount = 1, .pSubpasses = &sub, .dependencyCount = 1, .pDependencies = &dep};
    if (vkCreateRenderPass(dev, &rp, NULL, &s->pass) != VK_SUCCESS) goto fail;

    // The shadow pass: depth only, left ready for the scene to sample, and
    // waiting for the last frame's sampling before it writes again.
    VkAttachmentDescription sat = {0, VK_FORMAT_D16_UNORM, VK_SAMPLE_COUNT_1_BIT, VK_ATTACHMENT_LOAD_OP_CLEAR, VK_ATTACHMENT_STORE_OP_STORE,
        VK_ATTACHMENT_LOAD_OP_DONT_CARE, VK_ATTACHMENT_STORE_OP_DONT_CARE, VK_IMAGE_LAYOUT_UNDEFINED, VK_IMAGE_LAYOUT_DEPTH_STENCIL_READ_ONLY_OPTIMAL};
    VkAttachmentReference sdepth = {0, VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL};
    VkSubpassDescription ssub = {0, VK_PIPELINE_BIND_POINT_GRAPHICS, .pDepthStencilAttachment = &sdepth};
    VkSubpassDependency sdeps[2] = {
        {VK_SUBPASS_EXTERNAL, 0, VK_PIPELINE_STAGE_FRAGMENT_SHADER_BIT, VK_PIPELINE_STAGE_EARLY_FRAGMENT_TESTS_BIT,
         VK_ACCESS_SHADER_READ_BIT, VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_WRITE_BIT, 0},
        {0, VK_SUBPASS_EXTERNAL, VK_PIPELINE_STAGE_LATE_FRAGMENT_TESTS_BIT, VK_PIPELINE_STAGE_FRAGMENT_SHADER_BIT,
         VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT, 0},
    };
    VkRenderPassCreateInfo srp = {VK_STRUCTURE_TYPE_RENDER_PASS_CREATE_INFO, .attachmentCount = 1, .pAttachments = &sat,
        .subpassCount = 1, .pSubpasses = &ssub, .dependencyCount = 2, .pDependencies = sdeps};
    if (vkCreateRenderPass(dev, &srp, NULL, &s->shadow_pass) != VK_SUCCESS) goto fail;

    VkDescriptorSetLayoutBinding b[2] = {
        {0, VK_DESCRIPTOR_TYPE_UNIFORM_BUFFER, 1, VK_SHADER_STAGE_VERTEX_BIT | VK_SHADER_STAGE_FRAGMENT_BIT},
        {1, VK_DESCRIPTOR_TYPE_COMBINED_IMAGE_SAMPLER, 1, VK_SHADER_STAGE_FRAGMENT_BIT},
    };
    VkDescriptorSetLayoutCreateInfo dl = {VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO, .bindingCount = 2, .pBindings = b};
    if (vkCreateDescriptorSetLayout(dev, &dl, NULL, &s->dsl) != VK_SUCCESS) goto fail;
    VkDescriptorSetLayoutBinding tb = {0, VK_DESCRIPTOR_TYPE_COMBINED_IMAGE_SAMPLER, 1, VK_SHADER_STAGE_FRAGMENT_BIT};
    VkDescriptorSetLayoutCreateInfo tl = {VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO, .bindingCount = 1, .pBindings = &tb};
    if (vkCreateDescriptorSetLayout(dev, &tl, NULL, &s->tex_dsl) != VK_SUCCESS) goto fail;
    VkDescriptorSetLayout sets[2] = {s->dsl, s->tex_dsl};
    VkPushConstantRange pc = {VK_SHADER_STAGE_VERTEX_BIT | VK_SHADER_STAGE_FRAGMENT_BIT, 0, sizeof(sc_push)};
    VkPipelineLayoutCreateInfo pl = {VK_STRUCTURE_TYPE_PIPELINE_LAYOUT_CREATE_INFO, .setLayoutCount = 2, .pSetLayouts = sets,
        .pushConstantRangeCount = 1, .pPushConstantRanges = &pc};
    if (vkCreatePipelineLayout(dev, &pl, NULL, &s->layout) != VK_SUCCESS) goto fail;
    if (sc_pipeline(s, 0, VK_CULL_MODE_BACK_BIT, &s->pipeline) != VK_SUCCESS) goto fail;
    if (sc_pipeline(s, 0, VK_CULL_MODE_NONE, &s->pipeline_twosided) != VK_SUCCESS) goto fail;
    if (sc_pipeline(s, 1, VK_CULL_MODE_NONE, &s->shadow_pipeline) != VK_SUCCESS) goto fail;

    VkSamplerCreateInfo si = {VK_STRUCTURE_TYPE_SAMPLER_CREATE_INFO, .magFilter = VK_FILTER_LINEAR, .minFilter = VK_FILTER_LINEAR,
        .mipmapMode = VK_SAMPLER_MIPMAP_MODE_LINEAR, .addressModeU = VK_SAMPLER_ADDRESS_MODE_REPEAT, .addressModeV = VK_SAMPLER_ADDRESS_MODE_REPEAT,
        .addressModeW = VK_SAMPLER_ADDRESS_MODE_REPEAT, .maxLod = VK_LOD_CLAMP_NONE};
    if (vkCreateSampler(dev, &si, NULL, &s->sampler) != VK_SUCCESS) goto fail;
    // Comparing depth, point by point: every GPU does that for D16, and the
    // shader averages nine of them for a soft edge.
    VkSamplerCreateInfo ss = {VK_STRUCTURE_TYPE_SAMPLER_CREATE_INFO, .magFilter = VK_FILTER_NEAREST, .minFilter = VK_FILTER_NEAREST,
        .addressModeU = VK_SAMPLER_ADDRESS_MODE_CLAMP_TO_EDGE, .addressModeV = VK_SAMPLER_ADDRESS_MODE_CLAMP_TO_EDGE,
        .addressModeW = VK_SAMPLER_ADDRESS_MODE_CLAMP_TO_EDGE, .compareEnable = VK_TRUE, .compareOp = VK_COMPARE_OP_LESS_OR_EQUAL};
    if (vkCreateSampler(dev, &ss, NULL, &s->shadow_sampler) != VK_SUCCESS) goto fail;

    VkDescriptorPoolSize ps[2] = {{VK_DESCRIPTOR_TYPE_UNIFORM_BUFFER, SC_FRAMES}, {VK_DESCRIPTOR_TYPE_COMBINED_IMAGE_SAMPLER, SC_FRAMES}};
    VkDescriptorPoolCreateInfo dp = {VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO, .maxSets = SC_FRAMES, .poolSizeCount = 2, .pPoolSizes = ps};
    if (vkCreateDescriptorPool(dev, &dp, NULL, &s->pool) != VK_SUCCESS) goto fail;
    VkDescriptorSetLayout layouts[SC_FRAMES];
    for (int i = 0; i < SC_FRAMES; i++) layouts[i] = s->dsl;
    VkDescriptorSetAllocateInfo da = {VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO, .descriptorPool = s->pool, .descriptorSetCount = SC_FRAMES, .pSetLayouts = layouts};
    if (vkAllocateDescriptorSets(dev, &da, s->sets) != VK_SUCCESS) goto fail;
    VkCommandPoolCreateInfo cp = {VK_STRUCTURE_TYPE_COMMAND_POOL_CREATE_INFO, .flags = VK_COMMAND_POOL_CREATE_RESET_COMMAND_BUFFER_BIT, .queueFamilyIndex = family};
    if (vkCreateCommandPool(dev, &cp, NULL, &s->cmd_pool) != VK_SUCCESS) goto fail;
    VkCommandBufferAllocateInfo ca = {VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO, .commandPool = s->cmd_pool, .level = VK_COMMAND_BUFFER_LEVEL_PRIMARY, .commandBufferCount = SC_FRAMES};
    if (vkAllocateCommandBuffers(dev, &ca, s->cmds) != VK_SUCCESS) goto fail;

    if (sc_image2(s, SC_SHADOW, SC_SHADOW, 1, VK_SAMPLE_COUNT_1_BIT, VK_FORMAT_D16_UNORM,
                  VK_IMAGE_USAGE_DEPTH_STENCIL_ATTACHMENT_BIT | VK_IMAGE_USAGE_SAMPLED_BIT, VK_IMAGE_ASPECT_DEPTH_BIT, 0,
                  &s->shadow, &s->shadow_mem, &s->shadow_view) != VK_SUCCESS) goto fail;
    VkFramebufferCreateInfo sfi = {VK_STRUCTURE_TYPE_FRAMEBUFFER_CREATE_INFO, .renderPass = s->shadow_pass, .attachmentCount = 1,
        .pAttachments = &s->shadow_view, .width = SC_SHADOW, .height = SC_SHADOW, .layers = 1};
    if (vkCreateFramebuffer(dev, &sfi, NULL, &s->shadow_fb) != VK_SUCCESS) goto fail;

    for (int i = 0; i < SC_FRAMES; i++) {
        if (sc_buffer(s, sizeof(sc_ubo), VK_BUFFER_USAGE_UNIFORM_BUFFER_BIT, NULL, &s->ubo[i], &s->ubo_mem[i], &s->ubo_map[i]) != VK_SUCCESS) goto fail;
        VkDescriptorBufferInfo bi = {s->ubo[i], 0, sizeof(sc_ubo)};
        VkDescriptorImageInfo ii = {s->shadow_sampler, s->shadow_view, VK_IMAGE_LAYOUT_DEPTH_STENCIL_READ_ONLY_OPTIMAL};
        VkWriteDescriptorSet w[2] = {
            {VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET, .dstSet = s->sets[i], .dstBinding = 0, .descriptorCount = 1,
             .descriptorType = VK_DESCRIPTOR_TYPE_UNIFORM_BUFFER, .pBufferInfo = &bi},
            {VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET, .dstSet = s->sets[i], .dstBinding = 1, .descriptorCount = 1,
             .descriptorType = VK_DESCRIPTOR_TYPE_COMBINED_IMAGE_SAMPLER, .pImageInfo = &ii},
        };
        vkUpdateDescriptorSets(dev, 2, w, 0, NULL);
        VkFenceCreateInfo fi = {VK_STRUCTURE_TYPE_FENCE_CREATE_INFO, .flags = VK_FENCE_CREATE_SIGNALED_BIT};
        if (vkCreateFence(dev, &fi, NULL, &s->fences[i]) != VK_SUCCESS) goto fail;
    }
    uint32_t white = 0xffffffff;
    if (sc_make_texture(s, &s->white, &white, 1, 1) != VK_SUCCESS) goto fail;
    return s;
fail:
    sc_free(s);
    return NULL;
}

static void sc_drop_target(sc *s) {
    vkDeviceWaitIdle(s->dev);
    for (uint32_t i = 0; i < s->nviews && s->fbs; i++) vkDestroyFramebuffer(s->dev, s->fbs[i], NULL);
    free(s->fbs);
    free(s->views);
    s->fbs = NULL; s->views = NULL; s->nviews = 0;
    if (s->msaa_view) vkDestroyImageView(s->dev, s->msaa_view, NULL);
    if (s->msaa) vkDestroyImage(s->dev, s->msaa, NULL);
    if (s->msaa_mem) vkFreeMemory(s->dev, s->msaa_mem, NULL);
    if (s->depth_view) vkDestroyImageView(s->dev, s->depth_view, NULL);
    if (s->depth) vkDestroyImage(s->dev, s->depth, NULL);
    if (s->depth_mem) vkFreeMemory(s->dev, s->depth_mem, NULL);
    s->msaa_view = VK_NULL_HANDLE; s->msaa = VK_NULL_HANDLE; s->msaa_mem = VK_NULL_HANDLE;
    s->depth_view = VK_NULL_HANDLE; s->depth = VK_NULL_HANDLE; s->depth_mem = VK_NULL_HANDLE;
}

// sc_target draws into these views from now on: a swapchain's images.
static VkResult sc_target(sc *s, const VkImageView *views, uint32_t n, uint32_t w, uint32_t h) {
    sc_drop_target(s);
    s->width = w; s->height = h; s->nviews = n;
    s->views = malloc(n * sizeof *views);
    memcpy(s->views, views, n * sizeof *views);
    VkResult r = sc_image2(s, w, h, 1, VK_SAMPLE_COUNT_4_BIT, s->color_fmt, VK_IMAGE_USAGE_COLOR_ATTACHMENT_BIT | VK_IMAGE_USAGE_TRANSIENT_ATTACHMENT_BIT,
                           VK_IMAGE_ASPECT_COLOR_BIT, 1, &s->msaa, &s->msaa_mem, &s->msaa_view);
    if (r != VK_SUCCESS) return r;
    r = sc_image2(s, w, h, 1, VK_SAMPLE_COUNT_4_BIT, s->depth_fmt, VK_IMAGE_USAGE_DEPTH_STENCIL_ATTACHMENT_BIT | VK_IMAGE_USAGE_TRANSIENT_ATTACHMENT_BIT,
                  VK_IMAGE_ASPECT_DEPTH_BIT, 1, &s->depth, &s->depth_mem, &s->depth_view);
    if (r != VK_SUCCESS) return r;
    s->fbs = calloc(n, sizeof *s->fbs);
    for (uint32_t i = 0; i < n; i++) {
        VkImageView a[3] = {s->msaa_view, s->depth_view, views[i]};
        VkFramebufferCreateInfo fi = {VK_STRUCTURE_TYPE_FRAMEBUFFER_CREATE_INFO, .renderPass = s->pass, .attachmentCount = 3,
            .pAttachments = a, .width = w, .height = h, .layers = 1};
        if ((r = vkCreateFramebuffer(s->dev, &fi, NULL, &s->fbs[i])) != VK_SUCCESS) return r;
    }
    return VK_SUCCESS;
}

// ---- The scene's meshes and pictures, by key.

static void sc_free_mesh(sc *s, sc_mesh *m) {
    vkDestroyBuffer(s->dev, m->vb, NULL);
    vkFreeMemory(s->dev, m->vm, NULL);
    vkDestroyBuffer(s->dev, m->ib, NULL);
    vkFreeMemory(s->dev, m->im, NULL);
}

static int sc_find_mesh(sc *s, const char *key) {
    for (int i = 0; i < s->nmeshes; i++)
        if (strncmp(s->meshes[i].key, key, sizeof s->meshes[i].key) == 0) return i;
    return -1;
}

static int sc_find_texture(sc *s, const char *key) {
    for (int i = 0; i < s->ntextures; i++)
        if (strncmp(s->textures[i].key, key, sizeof s->textures[i].key) == 0) return i;
    return -1;
}

// sc_begin_assets starts an update: meshes and pictures not kept by the
// next sc_end_assets are freed.
static void sc_begin_assets(sc *s) {
    for (int i = 0; i < s->nmeshes; i++) s->meshes[i].used = 0;
    for (int i = 0; i < s->ntextures; i++) s->textures[i].used = 0;
}

static int sc_has_mesh(sc *s, const char *key) {
    int i = sc_find_mesh(s, key);
    if (i >= 0) s->meshes[i].used = 1;
    return i >= 0;
}

static int sc_has_texture(sc *s, const char *key) {
    int i = sc_find_texture(s, key);
    if (i >= 0) s->textures[i].used = 1;
    return i >= 0;
}

// sc_add_mesh takes vertices of 32 bytes (x y z, nx ny nz, u v) and count
// uint32 indices.
static VkResult sc_add_mesh(sc *s, const char *key, const void *vertices, uint32_t vbytes, const void *indices, uint32_t count) {
    if (sc_has_mesh(s, key)) return VK_SUCCESS;
    sc_mesh m = {0};
    strncpy(m.key, key, sizeof m.key - 1);
    VkResult r = sc_buffer(s, vbytes, VK_BUFFER_USAGE_VERTEX_BUFFER_BIT, vertices, &m.vb, &m.vm, NULL);
    if (r == VK_SUCCESS) r = sc_buffer(s, count * 4u, VK_BUFFER_USAGE_INDEX_BUFFER_BIT, indices, &m.ib, &m.im, NULL);
    if (r != VK_SUCCESS) return r;
    m.count = count;
    m.used = 1;
    s->meshes = realloc(s->meshes, (s->nmeshes + 1) * sizeof *s->meshes);
    s->meshes[s->nmeshes++] = m;
    return VK_SUCCESS;
}

// sc_add_texture takes w×h RGBA pixels, rows from the top.
static VkResult sc_add_texture(sc *s, const char *key, const void *rgba, uint32_t w, uint32_t h) {
    if (sc_has_texture(s, key)) return VK_SUCCESS;
    sc_texture t = {0};
    strncpy(t.key, key, sizeof t.key - 1);
    VkResult r = sc_make_texture(s, &t, rgba, w, h);
    if (r != VK_SUCCESS) { sc_free_texture(s, &t); return r; }
    t.used = 1;
    s->textures = realloc(s->textures, (s->ntextures + 1) * sizeof *s->textures);
    s->textures[s->ntextures++] = t;
    return VK_SUCCESS;
}

static void sc_end_assets(sc *s) {
    int kept = 0, waited = 0;
    for (int i = 0; i < s->nmeshes; i++) {
        if (s->meshes[i].used) { s->meshes[kept++] = s->meshes[i]; continue; }
        if (!waited++) vkDeviceWaitIdle(s->dev); // a frame in flight may use it
        sc_free_mesh(s, &s->meshes[i]);
    }
    s->nmeshes = kept;
    kept = 0;
    for (int i = 0; i < s->ntextures; i++) {
        if (s->textures[i].used) { s->textures[kept++] = s->textures[i]; continue; }
        if (!waited++) vkDeviceWaitIdle(s->dev);
        sc_free_texture(s, &s->textures[i]);
    }
    s->ntextures = kept;
}

// sc_scene sets the camera and light, as Scene.kt packs them:
//   clear rgba 4, eye 3, target 3, fov, near, far, ambient, shadows on,
//   the light's view 16, light count, then per light direction 3, color 3.
#define SC_SCENE_FLOATS 32
static void sc_scene(sc *s, const float *p, int n) {
    if (n < SC_SCENE_FLOATS) return;
    memcpy(s->clear, p, 4 * sizeof(float));
    memcpy(s->eye, p + 4, 3 * sizeof(float));
    memcpy(s->target, p + 7, 3 * sizeof(float));
    s->fov = p[10]; s->near_ = p[11]; s->far_ = p[12]; s->ambient = p[13];
    s->shadows = p[14] > 0.5f;
    memcpy(s->light_vp, p + 15, 16 * sizeof(float));
    s->nlights = (int)p[31];
    if (s->nlights > SC_LIGHTS) s->nlights = SC_LIGHTS;
    if (n < SC_SCENE_FLOATS + s->nlights * 6) s->nlights = (n - SC_SCENE_FLOATS) / 6;
    for (int i = 0; i < s->nlights; i++) {
        memcpy(s->light_dir[i], p + SC_SCENE_FLOATS + i * 6, 3 * sizeof(float));
        memcpy(s->light_col[i], p + SC_SCENE_FLOATS + 3 + i * 6, 3 * sizeof(float));
    }
}

// sc_camera moves the camera alone: what the orbit gesture does.
static void sc_camera(sc *s, const float *eye, const float *target) {
    memcpy(s->eye, eye, 3 * sizeof(float));
    memcpy(s->target, target, 3 * sizeof(float));
}

// Each node is SC_NODE_FLOATS floats: base 16, local 16, color 4, axis 3,
// speed, shine, unlit, two-sided, casts a shadow, and 1 + the index of the
// spinning node it turns with (0 for none).
#define SC_NODE_FLOATS 46

// sc_nodes replaces the nodes. A node that was spinning keeps its turn.
static void sc_nodes(sc *s, int n, const char *const *ids, const char *const *meshes, const char *const *textures, const float *p, double now) {
    sc_node *list = calloc(n > 0 ? n : 1, sizeof *list);
    for (int i = 0; i < n; i++) {
        sc_node *d = &list[i];
        const float *f = p + i * SC_NODE_FLOATS;
        strncpy(d->id, ids[i], sizeof d->id - 1);
        d->mesh = sc_find_mesh(s, meshes[i]);
        d->texture = textures[i][0] ? sc_find_texture(s, textures[i]) : -1;
        memcpy(d->base, f, 16 * sizeof(float));
        memcpy(d->local, f + 16, 16 * sizeof(float));
        memcpy(d->color, f + 32, 4 * sizeof(float));
        memcpy(d->axis, f + 36, 3 * sizeof(float));
        d->speed = f[39]; d->shine = f[40]; d->unlit = f[41]; d->twosided = f[42]; d->cast = f[43];
        d->parent = (int)f[44];
        d->started = now;
        for (int j = 0; j < s->nnodes; j++)
            if (d->id[0] && strcmp(s->nodes[j].id, d->id) == 0) d->started = s->nodes[j].started;
    }
    free(s->nodes);
    s->nodes = list;
    s->nnodes = n;
}

static int sc_spinning(sc *s) {
    for (int i = 0; i < s->nnodes; i++)
        if (s->nodes[i].speed != 0) return 1;
    return 0;
}

// sc_place works out where every node is at time now. Parents come first,
// so a node turning with a spinning one starts from where that one is.
static void sc_place(sc *s, double now) {
    for (int i = 0; i < s->nnodes; i++) {
        sc_node *n = &s->nodes[i];
        float spin[16];
        mat_axis_angle(spin, n->axis, (float)((now - n->started) * n->speed * M_PI / 180));
        mat_mul(n->world, n->base, spin);
        mat_mul(n->world, n->world, n->local);
        if (n->parent > 0 && n->parent <= i) mat_mul(n->world, s->nodes[n->parent - 1].world, n->world);
    }
}

static void sc_bind_mesh(VkCommandBuffer cb, sc_mesh *m) {
    VkDeviceSize off = 0;
    vkCmdBindVertexBuffers(cb, 0, 1, &m->vb, &off);
    vkCmdBindIndexBuffer(cb, m->ib, 0, VK_INDEX_TYPE_UINT32);
}

// sc_draw draws the scene at time now into the image-th view, waiting on
// wait (if any) and signalling done (if any) and the frame's fence.
static VkResult sc_draw(sc *s, uint32_t image, double now, VkSemaphore wait, VkSemaphore done) {
    uint32_t f = s->frame;
    s->frame = (s->frame + 1) % SC_FRAMES;
    vkWaitForFences(s->dev, 1, &s->fences[f], VK_TRUE, UINT64_MAX);
    vkResetFences(s->dev, 1, &s->fences[f]);
    sc_place(s, now);

    sc_ubo u = {0};
    float proj[16], view[16];
    mat_projection(proj, s->fov, (float)s->width / (float)(s->height ? s->height : 1), s->near_, s->far_);
    mat_look_at(view, s->eye, s->target);
    mat_mul(u.vp, proj, view);
    memcpy(u.light_vp, s->light_vp, sizeof u.light_vp);
    memcpy(u.eye, s->eye, 3 * sizeof(float));
    u.ambient[0] = s->ambient;
    u.ambient[1] = s->shadows ? 1 : 0;
    for (int i = 0; i < s->nlights; i++) {
        memcpy(u.dirs[i], s->light_dir[i], 3 * sizeof(float));
        memcpy(u.colors[i], s->light_col[i], 3 * sizeof(float));
    }
    u.count[0] = s->nlights;
    memcpy(s->ubo_map[f], &u, sizeof u);

    VkCommandBuffer cb = s->cmds[f];
    vkResetCommandBuffer(cb, 0);
    VkCommandBufferBeginInfo bi = {VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO, .flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT};
    vkBeginCommandBuffer(cb, &bi);
    vkCmdBindDescriptorSets(cb, VK_PIPELINE_BIND_POINT_GRAPHICS, s->layout, 0, 1, &s->sets[f], 0, NULL);

    // The shadow pass, always run so the map is in the layout the scene
    // samples; with shadows off it's just cleared.
    VkClearValue far_ = {.depthStencil = {1, 0}};
    VkRenderPassBeginInfo sb = {VK_STRUCTURE_TYPE_RENDER_PASS_BEGIN_INFO, .renderPass = s->shadow_pass, .framebuffer = s->shadow_fb,
        .renderArea = {{0, 0}, {SC_SHADOW, SC_SHADOW}}, .clearValueCount = 1, .pClearValues = &far_};
    vkCmdBeginRenderPass(cb, &sb, VK_SUBPASS_CONTENTS_INLINE);
    if (s->shadows) {
        vkCmdBindPipeline(cb, VK_PIPELINE_BIND_POINT_GRAPHICS, s->shadow_pipeline);
        VkViewport svp = {0, 0, SC_SHADOW, SC_SHADOW, 0, 1};
        VkRect2D ssc = {{0, 0}, {SC_SHADOW, SC_SHADOW}};
        vkCmdSetViewport(cb, 0, 1, &svp);
        vkCmdSetScissor(cb, 0, 1, &ssc);
        for (int i = 0; i < s->nnodes; i++) {
            sc_node *n = &s->nodes[i];
            if (!n->cast || n->mesh < 0 || n->mesh >= s->nmeshes) continue;
            sc_push p = {0};
            memcpy(p.model, n->world, sizeof p.model);
            vkCmdPushConstants(cb, s->layout, VK_SHADER_STAGE_VERTEX_BIT | VK_SHADER_STAGE_FRAGMENT_BIT, 0, sizeof p, &p);
            sc_bind_mesh(cb, &s->meshes[n->mesh]);
            vkCmdDrawIndexed(cb, s->meshes[n->mesh].count, 1, 0, 0, 0);
        }
    }
    vkCmdEndRenderPass(cb);

    VkClearValue clear[2] = {{.color = {{s->clear[0], s->clear[1], s->clear[2], s->clear[3]}}}, {.depthStencil = {1, 0}}};
    VkRenderPassBeginInfo rb = {VK_STRUCTURE_TYPE_RENDER_PASS_BEGIN_INFO, .renderPass = s->pass, .framebuffer = s->fbs[image],
        .renderArea = {{0, 0}, {s->width, s->height}}, .clearValueCount = 2, .pClearValues = clear};
    vkCmdBeginRenderPass(cb, &rb, VK_SUBPASS_CONTENTS_INLINE);
    VkViewport vp = {0, 0, (float)s->width, (float)s->height, 0, 1};
    VkRect2D sc_ = {{0, 0}, {s->width, s->height}};
    vkCmdSetViewport(cb, 0, 1, &vp);
    vkCmdSetScissor(cb, 0, 1, &sc_);
    VkPipeline bound = VK_NULL_HANDLE;
    for (int i = 0; i < s->nnodes; i++) {
        sc_node *n = &s->nodes[i];
        if (n->mesh < 0 || n->mesh >= s->nmeshes) continue;
        VkPipeline want = n->twosided ? s->pipeline_twosided : s->pipeline;
        if (want != bound) {
            vkCmdBindPipeline(cb, VK_PIPELINE_BIND_POINT_GRAPHICS, want);
            bound = want;
        }
        sc_texture *t = n->texture >= 0 && n->texture < s->ntextures ? &s->textures[n->texture] : &s->white;
        vkCmdBindDescriptorSets(cb, VK_PIPELINE_BIND_POINT_GRAPHICS, s->layout, 1, 1, &t->set, 0, NULL);
        sc_push p;
        memcpy(p.model, n->world, sizeof p.model);
        memcpy(p.color, n->color, sizeof p.color);
        p.params[0] = n->shine; p.params[1] = n->unlit; p.params[2] = p.params[3] = 0;
        vkCmdPushConstants(cb, s->layout, VK_SHADER_STAGE_VERTEX_BIT | VK_SHADER_STAGE_FRAGMENT_BIT, 0, sizeof p, &p);
        sc_bind_mesh(cb, &s->meshes[n->mesh]);
        vkCmdDrawIndexed(cb, s->meshes[n->mesh].count, 1, 0, 0, 0);
    }
    vkCmdEndRenderPass(cb);
    vkEndCommandBuffer(cb);
    VkPipelineStageFlags stage = VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT;
    VkSubmitInfo si = {VK_STRUCTURE_TYPE_SUBMIT_INFO, .waitSemaphoreCount = wait ? 1 : 0, .pWaitSemaphores = &wait,
        .pWaitDstStageMask = &stage, .commandBufferCount = 1, .pCommandBuffers = &cb,
        .signalSemaphoreCount = done ? 1 : 0, .pSignalSemaphores = &done};
    return vkQueueSubmit(s->queue, 1, &si, s->fences[f]);
}

static void sc_free(sc *s) {
    if (!s) return;
    if (s->dev) vkDeviceWaitIdle(s->dev);
    sc_drop_target(s);
    for (int i = 0; i < s->nmeshes; i++) sc_free_mesh(s, &s->meshes[i]);
    for (int i = 0; i < s->ntextures; i++) sc_free_texture(s, &s->textures[i]);
    sc_free_texture(s, &s->white);
    free(s->meshes);
    free(s->textures);
    free(s->nodes);
    for (int i = 0; i < SC_FRAMES; i++) {
        if (s->fences[i]) vkDestroyFence(s->dev, s->fences[i], NULL);
        if (s->ubo[i]) vkDestroyBuffer(s->dev, s->ubo[i], NULL);
        if (s->ubo_mem[i]) vkFreeMemory(s->dev, s->ubo_mem[i], NULL);
    }
    if (s->shadow_fb) vkDestroyFramebuffer(s->dev, s->shadow_fb, NULL);
    if (s->shadow_view) vkDestroyImageView(s->dev, s->shadow_view, NULL);
    if (s->shadow) vkDestroyImage(s->dev, s->shadow, NULL);
    if (s->shadow_mem) vkFreeMemory(s->dev, s->shadow_mem, NULL);
    if (s->sampler) vkDestroySampler(s->dev, s->sampler, NULL);
    if (s->shadow_sampler) vkDestroySampler(s->dev, s->shadow_sampler, NULL);
    if (s->cmd_pool) vkDestroyCommandPool(s->dev, s->cmd_pool, NULL);
    if (s->pool) vkDestroyDescriptorPool(s->dev, s->pool, NULL);
    if (s->pipeline) vkDestroyPipeline(s->dev, s->pipeline, NULL);
    if (s->pipeline_twosided) vkDestroyPipeline(s->dev, s->pipeline_twosided, NULL);
    if (s->shadow_pipeline) vkDestroyPipeline(s->dev, s->shadow_pipeline, NULL);
    if (s->layout) vkDestroyPipelineLayout(s->dev, s->layout, NULL);
    if (s->dsl) vkDestroyDescriptorSetLayout(s->dev, s->dsl, NULL);
    if (s->tex_dsl) vkDestroyDescriptorSetLayout(s->dev, s->tex_dsl, NULL);
    if (s->pass) vkDestroyRenderPass(s->dev, s->pass, NULL);
    if (s->shadow_pass) vkDestroyRenderPass(s->dev, s->shadow_pass, NULL);
    free(s);
}

#ifdef __ANDROID__
// ---- Android: a swapchain on a Surface, for Scene.kt.

#include <android/log.h>
#include <android/native_window_jni.h>
#include <jni.h>
#include <vulkan/vulkan_android.h>

#define LOG(...) __android_log_print(ANDROID_LOG_WARN, "nimbus-scene", __VA_ARGS__)

typedef struct {
    VkInstance inst;
    VkPhysicalDevice pd;
    VkDevice dev;
    VkQueue queue;
    uint32_t family;
    ANativeWindow *window;
    VkSurfaceKHR surface;
    VkSwapchainKHR swapchain;
    VkFormat format;
    uint32_t nimages;
    VkImage *images;
    VkImageView *views;
    VkSemaphore acquired[SC_FRAMES], rendered[SC_FRAMES];
    uint32_t sem;
    sc *scene;
} host;

static void host_drop_swapchain(host *h) {
    if (h->scene) sc_drop_target(h->scene);
    for (uint32_t i = 0; i < h->nimages; i++) vkDestroyImageView(h->dev, h->views[i], NULL);
    free(h->views); free(h->images);
    h->views = NULL; h->images = NULL; h->nimages = 0;
    if (h->swapchain) vkDestroySwapchainKHR(h->dev, h->swapchain, NULL);
    h->swapchain = VK_NULL_HANDLE;
}

static int host_swapchain(host *h) {
    vkDeviceWaitIdle(h->dev);
    VkSwapchainKHR old = h->swapchain;
    if (h->scene) sc_drop_target(h->scene);
    for (uint32_t i = 0; i < h->nimages; i++) vkDestroyImageView(h->dev, h->views[i], NULL);
    free(h->views); free(h->images);
    h->views = NULL; h->images = NULL; h->nimages = 0;

    VkSurfaceCapabilitiesKHR caps;
    vkGetPhysicalDeviceSurfaceCapabilitiesKHR(h->pd, h->surface, &caps);
    VkExtent2D ext = caps.currentExtent;
    if (ext.width == 0xFFFFFFFF) {
        ext.width = (uint32_t)ANativeWindow_getWidth(h->window);
        ext.height = (uint32_t)ANativeWindow_getHeight(h->window);
    }
    if (ext.width == 0 || ext.height == 0) return 0;
    uint32_t count = caps.minImageCount + 1;
    if (caps.maxImageCount && count > caps.maxImageCount) count = caps.maxImageCount;
    VkCompositeAlphaFlagBitsKHR alpha = VK_COMPOSITE_ALPHA_OPAQUE_BIT_KHR;
    if (!(caps.supportedCompositeAlpha & alpha)) alpha = VK_COMPOSITE_ALPHA_INHERIT_BIT_KHR;
    // The compositor turns the picture with the screen; it costs a little,
    // and keeps this simple.
    VkSurfaceTransformFlagBitsKHR transform = (caps.supportedTransforms & VK_SURFACE_TRANSFORM_IDENTITY_BIT_KHR)
        ? VK_SURFACE_TRANSFORM_IDENTITY_BIT_KHR : caps.currentTransform;
    VkSwapchainCreateInfoKHR ci = {VK_STRUCTURE_TYPE_SWAPCHAIN_CREATE_INFO_KHR, .surface = h->surface, .minImageCount = count,
        .imageFormat = h->format, .imageColorSpace = VK_COLOR_SPACE_SRGB_NONLINEAR_KHR, .imageExtent = ext, .imageArrayLayers = 1,
        .imageUsage = VK_IMAGE_USAGE_COLOR_ATTACHMENT_BIT, .imageSharingMode = VK_SHARING_MODE_EXCLUSIVE, .preTransform = transform,
        .compositeAlpha = alpha, .presentMode = VK_PRESENT_MODE_FIFO_KHR, .clipped = VK_TRUE, .oldSwapchain = old};
    VkResult r = vkCreateSwapchainKHR(h->dev, &ci, NULL, &h->swapchain);
    if (old) vkDestroySwapchainKHR(h->dev, old, NULL);
    if (r != VK_SUCCESS) { LOG("swapchain: %d", r); h->swapchain = VK_NULL_HANDLE; return 0; }
    vkGetSwapchainImagesKHR(h->dev, h->swapchain, &h->nimages, NULL);
    h->images = calloc(h->nimages, sizeof *h->images);
    h->views = calloc(h->nimages, sizeof *h->views);
    vkGetSwapchainImagesKHR(h->dev, h->swapchain, &h->nimages, h->images);
    for (uint32_t i = 0; i < h->nimages; i++) {
        VkImageViewCreateInfo vi = {VK_STRUCTURE_TYPE_IMAGE_VIEW_CREATE_INFO, .image = h->images[i], .viewType = VK_IMAGE_VIEW_TYPE_2D,
            .format = h->format, .subresourceRange = {VK_IMAGE_ASPECT_COLOR_BIT, 0, 1, 0, 1}};
        vkCreateImageView(h->dev, &vi, NULL, &h->views[i]);
    }
    return sc_target(h->scene, h->views, h->nimages, ext.width, ext.height) == VK_SUCCESS;
}

static void host_free(host *h) {
    if (!h) return;
    if (h->dev) {
        vkDeviceWaitIdle(h->dev);
        sc_free(h->scene);
        h->scene = NULL;
        host_drop_swapchain(h);
        for (int i = 0; i < SC_FRAMES; i++) {
            vkDestroySemaphore(h->dev, h->acquired[i], NULL);
            vkDestroySemaphore(h->dev, h->rendered[i], NULL);
        }
        vkDestroyDevice(h->dev, NULL);
    }
    if (h->surface) vkDestroySurfaceKHR(h->inst, h->surface, NULL);
    if (h->inst) vkDestroyInstance(h->inst, NULL);
    if (h->window) ANativeWindow_release(h->window);
    free(h);
}

static host *host_new(ANativeWindow *window) {
    host *h = calloc(1, sizeof *h);
    h->window = window;
    const char *iext[] = {VK_KHR_SURFACE_EXTENSION_NAME, VK_KHR_ANDROID_SURFACE_EXTENSION_NAME};
    VkApplicationInfo app = {VK_STRUCTURE_TYPE_APPLICATION_INFO, .pApplicationName = "nimbus-scene", .apiVersion = VK_API_VERSION_1_0};
    VkInstanceCreateInfo ic = {VK_STRUCTURE_TYPE_INSTANCE_CREATE_INFO, .pApplicationInfo = &app, .enabledExtensionCount = 2, .ppEnabledExtensionNames = iext};
    if (vkCreateInstance(&ic, NULL, &h->inst) != VK_SUCCESS) { LOG("no Vulkan"); goto fail; }
    VkAndroidSurfaceCreateInfoKHR sci = {VK_STRUCTURE_TYPE_ANDROID_SURFACE_CREATE_INFO_KHR, .window = window};
    if (vkCreateAndroidSurfaceKHR(h->inst, &sci, NULL, &h->surface) != VK_SUCCESS) goto fail;

    uint32_t n = 0;
    vkEnumeratePhysicalDevices(h->inst, &n, NULL);
    VkPhysicalDevice pds[8];
    if (n > 8) n = 8;
    vkEnumeratePhysicalDevices(h->inst, &n, pds);
    for (uint32_t i = 0; i < n && !h->pd; i++) {
        uint32_t nq = 0;
        vkGetPhysicalDeviceQueueFamilyProperties(pds[i], &nq, NULL);
        VkQueueFamilyProperties qs[16];
        if (nq > 16) nq = 16;
        vkGetPhysicalDeviceQueueFamilyProperties(pds[i], &nq, qs);
        for (uint32_t q = 0; q < nq; q++) {
            VkBool32 present = VK_FALSE;
            vkGetPhysicalDeviceSurfaceSupportKHR(pds[i], q, h->surface, &present);
            if ((qs[q].queueFlags & VK_QUEUE_GRAPHICS_BIT) && present) { h->pd = pds[i]; h->family = q; break; }
        }
    }
    if (!h->pd) { LOG("no GPU that can draw here"); goto fail; }
    float prio = 1;
    VkDeviceQueueCreateInfo qi = {VK_STRUCTURE_TYPE_DEVICE_QUEUE_CREATE_INFO, .queueFamilyIndex = h->family, .queueCount = 1, .pQueuePriorities = &prio};
    const char *dext[] = {VK_KHR_SWAPCHAIN_EXTENSION_NAME};
    VkDeviceCreateInfo dc = {VK_STRUCTURE_TYPE_DEVICE_CREATE_INFO, .queueCreateInfoCount = 1, .pQueueCreateInfos = &qi, .enabledExtensionCount = 1, .ppEnabledExtensionNames = dext};
    if (vkCreateDevice(h->pd, &dc, NULL, &h->dev) != VK_SUCCESS) goto fail;
    vkGetDeviceQueue(h->dev, h->family, 0, &h->queue);

    uint32_t nf = 0;
    vkGetPhysicalDeviceSurfaceFormatsKHR(h->pd, h->surface, &nf, NULL);
    VkSurfaceFormatKHR fs[32];
    if (nf > 32) nf = 32;
    vkGetPhysicalDeviceSurfaceFormatsKHR(h->pd, h->surface, &nf, fs);
    h->format = nf ? fs[0].format : VK_FORMAT_R8G8B8A8_UNORM;
    for (uint32_t i = 0; i < nf; i++)
        if (fs[i].format == VK_FORMAT_R8G8B8A8_UNORM) h->format = fs[i].format; // colors as Go gives them, like Metal's

    for (int i = 0; i < SC_FRAMES; i++) {
        VkSemaphoreCreateInfo si = {VK_STRUCTURE_TYPE_SEMAPHORE_CREATE_INFO};
        vkCreateSemaphore(h->dev, &si, NULL, &h->acquired[i]);
        vkCreateSemaphore(h->dev, &si, NULL, &h->rendered[i]);
    }
    h->scene = sc_new(h->pd, h->dev, h->family, h->queue, h->format, VK_IMAGE_LAYOUT_PRESENT_SRC_KHR);
    if (!h->scene) { LOG("pipeline"); goto fail; }
    if (!host_swapchain(h)) LOG("no swapchain yet");
    return h;
fail:
    host_free(h);
    return NULL;
}

// Returns 1 when drawn, 0 when there's nothing to draw on yet.
static int host_draw(host *h, double now) {
    if (!h->swapchain && !host_swapchain(h)) return 0;
    uint32_t sem = h->sem;
    h->sem = (h->sem + 1) % SC_FRAMES;
    uint32_t image;
    VkResult r = vkAcquireNextImageKHR(h->dev, h->swapchain, UINT64_MAX, h->acquired[sem], VK_NULL_HANDLE, &image);
    if (r == VK_ERROR_OUT_OF_DATE_KHR) return host_swapchain(h) ? host_draw(h, now) : 0;
    if (r != VK_SUCCESS && r != VK_SUBOPTIMAL_KHR) return 0;
    if (sc_draw(h->scene, image, now, h->acquired[sem], h->rendered[sem]) != VK_SUCCESS) return 0;
    VkPresentInfoKHR pi = {VK_STRUCTURE_TYPE_PRESENT_INFO_KHR, .waitSemaphoreCount = 1, .pWaitSemaphores = &h->rendered[sem],
        .swapchainCount = 1, .pSwapchains = &h->swapchain, .pImageIndices = &image};
    r = vkQueuePresentKHR(h->queue, &pi);
    if (r == VK_ERROR_OUT_OF_DATE_KHR || r == VK_SUBOPTIMAL_KHR) host_swapchain(h);
    return 1;
}

#define JNI(name) Java_space_nimbusgo_nativekit_SceneNative_##name

JNIEXPORT jlong JNICALL JNI(create)(JNIEnv *env, jclass cls, jobject surface) {
    ANativeWindow *w = ANativeWindow_fromSurface(env, surface);
    if (!w) return 0;
    return (jlong)(intptr_t)host_new(w);
}

JNIEXPORT void JNICALL JNI(resize)(JNIEnv *env, jclass cls, jlong p) {
    host *h = (host *)(intptr_t)p;
    if (h) host_swapchain(h);
}

JNIEXPORT void JNICALL JNI(destroy)(JNIEnv *env, jclass cls, jlong p) { host_free((host *)(intptr_t)p); }

JNIEXPORT void JNICALL JNI(beginAssets)(JNIEnv *env, jclass cls, jlong p) { sc_begin_assets(((host *)(intptr_t)p)->scene); }
JNIEXPORT void JNICALL JNI(endAssets)(JNIEnv *env, jclass cls, jlong p) { sc_end_assets(((host *)(intptr_t)p)->scene); }

JNIEXPORT jboolean JNICALL JNI(hasMesh)(JNIEnv *env, jclass cls, jlong p, jstring key) {
    const char *k = (*env)->GetStringUTFChars(env, key, NULL);
    int has = sc_has_mesh(((host *)(intptr_t)p)->scene, k);
    (*env)->ReleaseStringUTFChars(env, key, k);
    return has ? JNI_TRUE : JNI_FALSE;
}

JNIEXPORT jboolean JNICALL JNI(hasTexture)(JNIEnv *env, jclass cls, jlong p, jstring key) {
    const char *k = (*env)->GetStringUTFChars(env, key, NULL);
    int has = sc_has_texture(((host *)(intptr_t)p)->scene, k);
    (*env)->ReleaseStringUTFChars(env, key, k);
    return has ? JNI_TRUE : JNI_FALSE;
}

JNIEXPORT void JNICALL JNI(addMesh)(JNIEnv *env, jclass cls, jlong p, jstring key, jbyteArray vertices, jbyteArray indices) {
    const char *k = (*env)->GetStringUTFChars(env, key, NULL);
    jsize vn = (*env)->GetArrayLength(env, vertices), in = (*env)->GetArrayLength(env, indices);
    jbyte *v = (*env)->GetByteArrayElements(env, vertices, NULL), *i = (*env)->GetByteArrayElements(env, indices, NULL);
    if (vn >= 32 && in >= 4) sc_add_mesh(((host *)(intptr_t)p)->scene, k, v, (uint32_t)vn, i, (uint32_t)in / 4);
    (*env)->ReleaseByteArrayElements(env, vertices, v, JNI_ABORT);
    (*env)->ReleaseByteArrayElements(env, indices, i, JNI_ABORT);
    (*env)->ReleaseStringUTFChars(env, key, k);
}

JNIEXPORT void JNICALL JNI(addTexture)(JNIEnv *env, jclass cls, jlong p, jstring key, jbyteArray rgba, jint w, jint h) {
    const char *k = (*env)->GetStringUTFChars(env, key, NULL);
    jsize n = (*env)->GetArrayLength(env, rgba);
    jbyte *px = (*env)->GetByteArrayElements(env, rgba, NULL);
    if (w > 0 && h > 0 && n >= w * h * 4) sc_add_texture(((host *)(intptr_t)p)->scene, k, px, (uint32_t)w, (uint32_t)h);
    (*env)->ReleaseByteArrayElements(env, rgba, px, JNI_ABORT);
    (*env)->ReleaseStringUTFChars(env, key, k);
}

JNIEXPORT void JNICALL JNI(scene)(JNIEnv *env, jclass cls, jlong p, jfloatArray params) {
    jsize n = (*env)->GetArrayLength(env, params);
    jfloat *f = (*env)->GetFloatArrayElements(env, params, NULL);
    sc_scene(((host *)(intptr_t)p)->scene, f, n);
    (*env)->ReleaseFloatArrayElements(env, params, f, JNI_ABORT);
}

JNIEXPORT void JNICALL JNI(camera)(JNIEnv *env, jclass cls, jlong p, jfloatArray eyeTarget) {
    if ((*env)->GetArrayLength(env, eyeTarget) < 6) return;
    jfloat *f = (*env)->GetFloatArrayElements(env, eyeTarget, NULL);
    sc_camera(((host *)(intptr_t)p)->scene, f, f + 3);
    (*env)->ReleaseFloatArrayElements(env, eyeTarget, f, JNI_ABORT);
}

JNIEXPORT void JNICALL JNI(nodes)(JNIEnv *env, jclass cls, jlong p, jobjectArray ids, jobjectArray meshes, jobjectArray textures, jfloatArray data, jdouble now) {
    jsize n = (*env)->GetArrayLength(env, ids);
    if ((*env)->GetArrayLength(env, data) < n * SC_NODE_FLOATS) return;
    jsize m = n ? n : 1;
    const char **strs = calloc(3 * m, sizeof *strs);
    jstring *js = calloc(3 * m, sizeof *js);
    jobjectArray arrays[3] = {ids, meshes, textures};
    for (jsize i = 0; i < n; i++)
        for (int a = 0; a < 3; a++) {
            js[a * m + i] = (jstring)(*env)->GetObjectArrayElement(env, arrays[a], i);
            strs[a * m + i] = (*env)->GetStringUTFChars(env, js[a * m + i], NULL);
        }
    jfloat *f = (*env)->GetFloatArrayElements(env, data, NULL);
    sc_nodes(((host *)(intptr_t)p)->scene, n, strs, strs + m, strs + 2 * m, f, now);
    (*env)->ReleaseFloatArrayElements(env, data, f, JNI_ABORT);
    for (jsize i = 0; i < 3 * n; i++) {
        jsize a = i / n, k = i % n;
        (*env)->ReleaseStringUTFChars(env, js[a * m + k], strs[a * m + k]);
        (*env)->DeleteLocalRef(env, js[a * m + k]);
    }
    free(strs); free(js);
}

JNIEXPORT jboolean JNICALL JNI(spinning)(JNIEnv *env, jclass cls, jlong p) {
    return sc_spinning(((host *)(intptr_t)p)->scene) ? JNI_TRUE : JNI_FALSE;
}

JNIEXPORT jboolean JNICALL JNI(draw)(JNIEnv *env, jclass cls, jlong p, jdouble now) {
    return host_draw((host *)(intptr_t)p, now) ? JNI_TRUE : JNI_FALSE;
}
#endif
