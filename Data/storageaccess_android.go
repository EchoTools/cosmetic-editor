//go:build android

package data

/*
#include <jni.h>
#include <stdint.h>
#include <stdlib.h>

// Returns 1 if the app holds "All files access", 0 if not, -1 if the platform
// does not have the concept (before Android 11).
static int evr_is_storage_manager(uintptr_t envp) {
	JNIEnv* env = (JNIEnv*)envp;
	jclass cls = (*env)->FindClass(env, "android/os/Environment");
	if (cls == NULL) { (*env)->ExceptionClear(env); return -1; }
	jmethodID m = (*env)->GetStaticMethodID(env, cls, "isExternalStorageManager", "()Z");
	if (m == NULL) {
		(*env)->ExceptionClear(env);
		(*env)->DeleteLocalRef(env, cls);
		return -1;
	}
	jboolean r = (*env)->CallStaticBooleanMethod(env, cls, m);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); r = JNI_FALSE; }
	(*env)->DeleteLocalRef(env, cls);
	return r ? 1 : 0;
}

// Opens the system screen where the user can switch on "All files access" for
// this app. Returns 0 on success, non-zero if the screen could not be opened.
// Any Java exception is cleared here: an uncaught one would crash the app.
static int evr_open_all_files_settings(uintptr_t envp, uintptr_t ctxp, const char* pkgURI) {
	JNIEnv* env = (JNIEnv*)envp;
	jobject ctx = (jobject)ctxp;
	int rc = 1;

	jclass uriCls = (*env)->FindClass(env, "android/net/Uri");
	jclass intentCls = (*env)->FindClass(env, "android/content/Intent");
	if (uriCls == NULL || intentCls == NULL) goto done;

	jmethodID parse = (*env)->GetStaticMethodID(env, uriCls, "parse", "(Ljava/lang/String;)Landroid/net/Uri;");
	jmethodID ctor = (*env)->GetMethodID(env, intentCls, "<init>", "(Ljava/lang/String;Landroid/net/Uri;)V");
	jmethodID addFlags = (*env)->GetMethodID(env, intentCls, "addFlags", "(I)Landroid/content/Intent;");
	if (parse == NULL || ctor == NULL || addFlags == NULL) goto done;

	jstring uriStr = (*env)->NewStringUTF(env, pkgURI);
	jobject uri = (*env)->CallStaticObjectMethod(env, uriCls, parse, uriStr);
	if ((*env)->ExceptionCheck(env) || uri == NULL) goto done;

	jstring action = (*env)->NewStringUTF(env, "android.settings.MANAGE_APP_ALL_FILES_ACCESS_PERMISSION");
	jobject intent = (*env)->NewObject(env, intentCls, ctor, action, uri);
	if ((*env)->ExceptionCheck(env) || intent == NULL) goto done;
	(*env)->CallObjectMethod(env, intent, addFlags, (jint)0x10000000); // FLAG_ACTIVITY_NEW_TASK
	if ((*env)->ExceptionCheck(env)) goto done;

	jclass ctxCls = (*env)->GetObjectClass(env, ctx);
	jmethodID start = (*env)->GetMethodID(env, ctxCls, "startActivity", "(Landroid/content/Intent;)V");
	if (start == NULL) goto done;
	(*env)->CallVoidMethod(env, ctx, start, intent);
	if ((*env)->ExceptionCheck(env)) goto done;
	rc = 0;

done:
	if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
	return rc;
}
*/
import "C"

import (
	"errors"
	"unsafe"

	"fyne.io/fyne/v2/driver"
)

// HasAllFilesAccess reports whether the app may read and write other apps'
// files, which it needs because Echo VR's data belongs to the game.
func HasAllFilesAccess() bool {
	granted := false
	driver.RunNative(func(c any) error {
		if ac, ok := c.(*driver.AndroidContext); ok {
			// Before Android 11 there is no such permission; the legacy storage
			// permissions cover it, so treat access as granted.
			granted = C.evr_is_storage_manager(C.uintptr_t(ac.Env)) != 0
		}
		return nil
	})
	return granted
}

// RequestAllFilesAccess opens the system screen where the user switches on
// "All files access" for this app. The app cannot grant this to itself.
func RequestAllFilesAccess(appID string) error {
	uri := C.CString("package:" + appID)
	defer C.free(unsafe.Pointer(uri))
	var rc C.int = 1
	err := driver.RunNative(func(c any) error {
		ac, ok := c.(*driver.AndroidContext)
		if !ok {
			return errors.New("not running on Android")
		}
		rc = C.evr_open_all_files_settings(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx), uri)
		return nil
	})
	if err != nil {
		return err
	}
	if rc != 0 {
		return errors.New("could not open the All files access settings screen")
	}
	return nil
}
