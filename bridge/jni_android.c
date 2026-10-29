// JNI glue for Android hosts. It binds the C ABI in nn.h to the Java class
// space.nimbusgo.nativekit.Bridge — no generated code, no gomobile.

#include <jni.h>
#include <stdio.h>
#include <stdlib.h>
#include "nn.h"

#define BRIDGE_CLASS "space/nimbusgo/nativekit/Bridge"
#define EXCEPTION_CLASS "space/nimbusgo/nativekit/BridgeException"

static JavaVM *g_vm;
static jclass g_bridge;       // global refs, resolved on the loader thread:
static jclass g_exception;    // FindClass on a Go thread can't see app classes
static jmethodID g_on_wake;   // static void Bridge.onWake()
static jmethodID g_exc_init;  // BridgeException(int, byte[])

// Go calls wake from its own threads; attach them as daemons so they never
// need detaching and never keep the VM alive.
static JNIEnv *env_for_thread(void) {
	JNIEnv *env = NULL;
	jint st = (*g_vm)->GetEnv(g_vm, (void **)&env, JNI_VERSION_1_6);
	if (st == JNI_EDETACHED && (*g_vm)->AttachCurrentThreadAsDaemon(g_vm, &env, NULL) != JNI_OK) {
		return NULL;
	}
	return env;
}

static void jni_wake(void *ctx) {
	JNIEnv *env = env_for_thread();
	if (env == NULL) return;
	(*env)->CallStaticVoidMethod(env, g_bridge, g_on_wake);
	if ((*env)->ExceptionCheck(env)) {
		(*env)->ExceptionDescribe(env);
		(*env)->ExceptionClear(env);
	}
}

static jbyteArray to_java(JNIEnv *env, const uint8_t *p, int32_t n) {
	jbyteArray arr = (*env)->NewByteArray(env, n);
	if (arr != NULL && n > 0) (*env)->SetByteArrayRegion(env, arr, 0, n, (const jbyte *)p);
	return arr;
}

// Throws BridgeException(code, detail). Takes ownership of detail.
static void throw_bridge(JNIEnv *env, jint code, uint8_t *detail, int32_t n) {
	jbyteArray d = detail != NULL ? to_java(env, detail, n) : NULL;
	if (detail != NULL) nn_free(detail);
	jobject ex = (*env)->NewObject(env, g_exception, g_exc_init, code, d);
	if (ex != NULL) (*env)->Throw(env, (jthrowable)ex);
}

static jint Bridge_start(JNIEnv *env, jclass cls) {
	static nn_host host;
	host.abi = NN_ABI_VERSION;
	host.ctx = NULL;
	host.wake = jni_wake;
	return nn_start(&host);
}

static jint Bridge_send(JNIEnv *env, jclass cls, jbyteArray msg) {
	jsize n = (*env)->GetArrayLength(env, msg);
	jbyte *p = (*env)->GetByteArrayElements(env, msg, NULL);
	jint rc = nn_send((const uint8_t *)p, n);
	(*env)->ReleaseByteArrayElements(env, msg, p, JNI_ABORT);
	return rc;
}

static jbyteArray Bridge_drain(JNIEnv *env, jclass cls) {
	uint8_t *out = NULL;
	int32_t n = 0;
	jint rc = nn_drain(&out, &n);
	if (rc != NN_OK) {
		throw_bridge(env, rc, out, n);
		return NULL;
	}
	if (out == NULL) return NULL;
	jbyteArray arr = to_java(env, out, n);
	nn_free(out);
	return arr;
}

static jbyteArray Bridge_call(JNIEnv *env, jclass cls, jbyteArray req) {
	jsize n = (*env)->GetArrayLength(env, req);
	jbyte *p = (*env)->GetByteArrayElements(env, req, NULL);
	uint8_t *out = NULL;
	int32_t out_len = 0;
	jint rc = nn_call((const uint8_t *)p, n, &out, &out_len);
	(*env)->ReleaseByteArrayElements(env, req, p, JNI_ABORT);

	if (rc != NN_OK) {
		throw_bridge(env, rc, out, out_len); // panic text and Go stack travel with the exception
		return NULL;
	}
	jbyteArray arr = to_java(env, out, out_len);
	if (out != NULL) nn_free(out);
	return arr;
}

JNIEXPORT jint JNI_OnLoad(JavaVM *vm, void *reserved) {
	g_vm = vm;
	JNIEnv *env;
	if ((*vm)->GetEnv(vm, (void **)&env, JNI_VERSION_1_6) != JNI_OK) return JNI_ERR;

	jclass cls = (*env)->FindClass(env, BRIDGE_CLASS);
	jclass exc = (*env)->FindClass(env, EXCEPTION_CLASS);
	if (cls == NULL || exc == NULL) return JNI_ERR;
	g_bridge = (*env)->NewGlobalRef(env, cls);
	g_exception = (*env)->NewGlobalRef(env, exc);
	g_on_wake = (*env)->GetStaticMethodID(env, cls, "onWake", "()V");
	g_exc_init = (*env)->GetMethodID(env, exc, "<init>", "(I[B)V");
	if (g_on_wake == NULL || g_exc_init == NULL) return JNI_ERR;

	static const JNINativeMethod methods[] = {
		{"nativeStart", "()I", (void *)Bridge_start},
		{"nativeSend", "([B)I", (void *)Bridge_send},
		{"nativeDrain", "()[B", (void *)Bridge_drain},
		{"nativeCall", "([B)[B", (void *)Bridge_call},
	};
	if ((*env)->RegisterNatives(env, cls, methods, sizeof methods / sizeof methods[0]) != JNI_OK) return JNI_ERR;
	(*env)->DeleteLocalRef(env, cls);
	(*env)->DeleteLocalRef(env, exc);
	return JNI_VERSION_1_6;
}
