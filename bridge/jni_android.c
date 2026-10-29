// JNI glue for Android hosts. It binds the C ABI in nn.h to the Java class
// space.nimbusgo.nativekit.Bridge — no generated code, no gomobile.

#include <jni.h>
#include <stdio.h>
#include <stdlib.h>
#include "nn.h"

#define BRIDGE_CLASS "space/nimbusgo/nativekit/Bridge"

static JavaVM *g_vm;
static jclass g_bridge;          // global ref, resolved on the loader thread
static jmethodID g_on_receive;   // static void onReceive(byte[])

// Go calls back from its own threads; attach them as daemons so they never
// need detaching and never keep the VM alive.
static JNIEnv *env_for_thread(void) {
	JNIEnv *env = NULL;
	jint st = (*g_vm)->GetEnv(g_vm, (void **)&env, JNI_VERSION_1_6);
	if (st == JNI_EDETACHED && (*g_vm)->AttachCurrentThreadAsDaemon(g_vm, &env, NULL) != JNI_OK) {
		return NULL;
	}
	return env;
}

static void jni_receive(void *ctx, const uint8_t *data, int32_t len) {
	JNIEnv *env = env_for_thread();
	if (env == NULL) return;
	jbyteArray arr = (*env)->NewByteArray(env, len);
	if (arr == NULL) { (*env)->ExceptionClear(env); return; }
	if (len > 0) (*env)->SetByteArrayRegion(env, arr, 0, len, (const jbyte *)data);
	(*env)->CallStaticVoidMethod(env, g_bridge, g_on_receive, arr);
	if ((*env)->ExceptionCheck(env)) {
		(*env)->ExceptionDescribe(env);
		(*env)->ExceptionClear(env);
	}
	(*env)->DeleteLocalRef(env, arr);
}

static jint Bridge_start(JNIEnv *env, jclass cls) {
	static nn_host host;
	host.abi = NN_ABI_VERSION;
	host.ctx = NULL;
	host.receive = jni_receive;
	return nn_start(&host);
}

static jint Bridge_send(JNIEnv *env, jclass cls, jbyteArray msg) {
	jsize n = (*env)->GetArrayLength(env, msg);
	jbyte *p = (*env)->GetByteArrayElements(env, msg, NULL);
	jint rc = nn_send((const uint8_t *)p, n);
	(*env)->ReleaseByteArrayElements(env, msg, p, JNI_ABORT);
	return rc;
}

static jbyteArray Bridge_call(JNIEnv *env, jclass cls, jbyteArray req) {
	jsize n = (*env)->GetArrayLength(env, req);
	jbyte *p = (*env)->GetByteArrayElements(env, req, NULL);
	uint8_t *out = NULL;
	int32_t out_len = 0;
	jint rc = nn_call((const uint8_t *)p, n, &out, &out_len);
	(*env)->ReleaseByteArrayElements(env, req, p, JNI_ABORT);

	if (rc != NN_OK) {
		char msg[64];
		snprintf(msg, sizeof msg, "nimbus-native: call failed (code %d)", rc);
		jclass ex = (*env)->FindClass(env, "java/lang/IllegalStateException");
		if (out != NULL) nn_free(out); // panic text is already in logcat
		(*env)->ThrowNew(env, ex, msg);
		return NULL;
	}
	jbyteArray arr = (*env)->NewByteArray(env, out_len);
	if (arr != NULL && out_len > 0) (*env)->SetByteArrayRegion(env, arr, 0, out_len, (const jbyte *)out);
	if (out != NULL) nn_free(out);
	return arr;
}

JNIEXPORT jint JNI_OnLoad(JavaVM *vm, void *reserved) {
	g_vm = vm;
	JNIEnv *env;
	if ((*vm)->GetEnv(vm, (void **)&env, JNI_VERSION_1_6) != JNI_OK) return JNI_ERR;

	jclass cls = (*env)->FindClass(env, BRIDGE_CLASS);
	if (cls == NULL) return JNI_ERR;
	g_bridge = (*env)->NewGlobalRef(env, cls);
	g_on_receive = (*env)->GetStaticMethodID(env, cls, "onReceive", "([B)V");
	if (g_on_receive == NULL) return JNI_ERR;

	static const JNINativeMethod methods[] = {
		{"nativeStart", "()I", (void *)Bridge_start},
		{"nativeSend", "([B)I", (void *)Bridge_send},
		{"nativeCall", "([B)[B", (void *)Bridge_call},
	};
	if ((*env)->RegisterNatives(env, cls, methods, sizeof methods / sizeof methods[0]) != JNI_OK) return JNI_ERR;
	(*env)->DeleteLocalRef(env, cls);
	return JNI_VERSION_1_6;
}
