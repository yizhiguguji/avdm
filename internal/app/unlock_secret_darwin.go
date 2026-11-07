package app

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <Security/Security.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>
#include <string.h>
static CFMutableDictionaryRef adm_unlock_query(const char *account) {
 CFMutableDictionaryRef q=CFDictionaryCreateMutable(NULL,0,&kCFTypeDictionaryKeyCallBacks,&kCFTypeDictionaryValueCallBacks);
 CFStringRef a=CFStringCreateWithCString(NULL,account,kCFStringEncodingUTF8);
 CFDictionarySetValue(q,kSecClass,kSecClassGenericPassword);
 CFDictionarySetValue(q,kSecAttrService,CFSTR("com.zhuiguang.advm.device-unlock"));
 CFDictionarySetValue(q,kSecAttrAccount,a); CFRelease(a);return q;
}
static int adm_unlock_save(const char *account,const char *value) {
 CFMutableDictionaryRef q=adm_unlock_query(account);
 CFDataRef data=CFDataCreate(NULL,(const UInt8*)value,strlen(value));
 CFDictionaryRef attrs=CFDictionaryCreate(NULL,(const void*[]){kSecValueData},(const void*[]){data},1,&kCFTypeDictionaryKeyCallBacks,&kCFTypeDictionaryValueCallBacks);
 OSStatus s=SecItemUpdate(q,attrs);
 if(s==errSecItemNotFound){CFDictionarySetValue(q,kSecValueData,data);s=SecItemAdd(q,NULL);}
 CFRelease(attrs);CFRelease(data);CFRelease(q);return (int)s;
}
static int adm_unlock_read(const char *account,char **out) {
 CFMutableDictionaryRef q=adm_unlock_query(account);CFDictionarySetValue(q,kSecReturnData,kCFBooleanTrue);
 CFTypeRef result=NULL;OSStatus s=SecItemCopyMatching(q,&result);CFRelease(q);
 if(s==errSecSuccess){CFDataRef data=(CFDataRef)result;CFIndex n=CFDataGetLength(data);*out=calloc(n+1,1);memcpy(*out,CFDataGetBytePtr(data),n);CFRelease(result);}return (int)s;
}
static int adm_unlock_delete(const char *account){CFMutableDictionaryRef q=adm_unlock_query(account);OSStatus s=SecItemDelete(q);CFRelease(q);return (int)s;}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

func unlockSecretRead(account string) (string, error) {
	a := C.CString(account)
	defer C.free(unsafe.Pointer(a))
	var out *C.char
	s := C.adm_unlock_read(a, &out)
	if s == -25300 {
		return "", nil
	}
	if s != 0 {
		return "", fmt.Errorf("读取解锁钥匙串失败（%d）", int(s))
	}
	defer C.free(unsafe.Pointer(out))
	return C.GoString(out), nil
}
func unlockSecretSave(account, value string) error {
	a := C.CString(account)
	v := C.CString(value)
	defer C.free(unsafe.Pointer(a))
	defer C.free(unsafe.Pointer(v))
	if s := C.adm_unlock_save(a, v); s != 0 {
		return fmt.Errorf("保存解锁钥匙串失败（%d）", int(s))
	}
	return nil
}
func unlockSecretDelete(account string) error {
	a := C.CString(account)
	defer C.free(unsafe.Pointer(a))
	if s := C.adm_unlock_delete(a); s != 0 && s != -25300 {
		return fmt.Errorf("删除解锁钥匙串失败（%d）", int(s))
	}
	return nil
}
