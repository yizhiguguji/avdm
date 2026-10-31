//go:build darwin

package app

/*
#cgo darwin LDFLAGS: -framework ApplicationServices -framework CoreGraphics
#include <ApplicationServices/ApplicationServices.h>
#include <CoreGraphics/CoreGraphics.h>
#include <string.h>
#include <math.h>
#include <stdlib.h>
static int adm_ax_text_contains(AXUIElementRef node, CFStringRef attribute, const char* needle);

static int adm_emulator_window_top(int pid, const char* avd) {
 // Resolve the actual AVD window through AX before consulting its public CG
 // layer. Extended Controls can be larger than the device and must not win.
 AXUIElementRef app=AXUIElementCreateApplication(pid);
 CFTypeRef windows=NULL;
 CGRect target=CGRectZero;
 if (AXUIElementCopyAttributeValue(app,kAXWindowsAttribute,&windows)==kAXErrorSuccess && windows && CFGetTypeID(windows)==CFArrayGetTypeID()) {
  for (CFIndex i=0;i<CFArrayGetCount(windows);i++) {
   AXUIElementRef win=CFArrayGetValueAtIndex(windows,i);
   if (!adm_ax_text_contains(win,kAXTitleAttribute,avd) || adm_ax_text_contains(win,kAXTitleAttribute,"Extended controls")) continue;
   CFTypeRef p=NULL,s=NULL;
   AXUIElementCopyAttributeValue(win,kAXPositionAttribute,&p);
   AXUIElementCopyAttributeValue(win,kAXSizeAttribute,&s);
   CGPoint pos=CGPointZero;CGSize size=CGSizeZero;
   if (p && s && CFGetTypeID(p)==AXValueGetTypeID() && CFGetTypeID(s)==AXValueGetTypeID() && AXValueGetValue(p,kAXValueCGPointType,&pos) && AXValueGetValue(s,kAXValueCGSizeType,&size) && size.width*size.height>target.size.width*target.size.height) target=CGRectMake(pos.x,pos.y,size.width,size.height);
   if (p) CFRelease(p);if (s) CFRelease(s);
  }
 }
 if (windows) CFRelease(windows);CFRelease(app);
 if (target.size.width<=0) return -1;
 CFArrayRef infos = CGWindowListCopyWindowInfo(kCGWindowListOptionAll, kCGNullWindowID);
 if (!infos) return -1;
 double largest = 0; int result = -1;
 for (CFIndex i=0; i<CFArrayGetCount(infos); i++) {
  CFDictionaryRef info = CFArrayGetValueAtIndex(infos,i);
  CFNumberRef owner = CFDictionaryGetValue(info,kCGWindowOwnerPID);
  int ownerPID=0, layer=0;
  if (!owner || !CFNumberGetValue(owner,kCFNumberIntType,&ownerPID) || ownerPID!=pid) continue;
  CFNumberRef level = CFDictionaryGetValue(info,kCGWindowLayer);
  if (level) CFNumberGetValue(level,kCFNumberIntType,&layer);
  if (layer<0 || layer>100) continue;
  CGRect bounds=CGRectZero;
  CFDictionaryRef value=CFDictionaryGetValue(info,kCGWindowBounds);
  if (!value || !CGRectMakeWithDictionaryRepresentation(value,&bounds) || bounds.size.height<=32) continue;
  if (fabs(bounds.origin.x-target.origin.x)>3 || fabs(bounds.origin.y-target.origin.y)>3 || fabs(bounds.size.width-target.size.width)>3 || fabs(bounds.size.height-target.size.height)>3) continue;
  double area=bounds.size.width*bounds.size.height;
  if (area>largest) {largest=area;result=layer>0;}
 }
 CFRelease(infos);
 return result;
}

static int adm_ax_text_contains(AXUIElementRef node, CFStringRef attribute, const char* needle) {
 CFTypeRef value=NULL;
 if (AXUIElementCopyAttributeValue(node,attribute,&value)!=kAXErrorSuccess || !value) return 0;
 int found=0;
 if (CFGetTypeID(value)==CFStringGetTypeID()) {
  CFStringRef text=CFStringCreateWithCString(NULL,needle,kCFStringEncodingUTF8);
  found=CFStringFind(value,text,kCFCompareCaseInsensitive).location!=kCFNotFound;
  CFRelease(text);
 }
 CFRelease(value);return found;
}

// Only the vendor's specifically named switch is eligible. Never toggle an
// arbitrary checkbox based on its position or sibling order.
static AXUIElementRef adm_find_top_switch(AXUIElementRef node, int depth, int* budget) {
 if (depth>24 || --*budget<0) return NULL;
 CFTypeRef role=NULL;
 AXUIElementCopyAttributeValue(node,kAXRoleAttribute,&role);
 int checkbox=role && CFGetTypeID(role)==CFStringGetTypeID() && CFEqual(role,kAXCheckBoxRole);
 if (role) CFRelease(role);
 if (checkbox && (adm_ax_text_contains(node,CFSTR("AXIdentifier"),"set_onTop") ||
                  adm_ax_text_contains(node,kAXHelpAttribute,"Emulator always on top") ||
                  adm_ax_text_contains(node,kAXDescriptionAttribute,"Emulator always on top") ||
                  adm_ax_text_contains(node,kAXTitleAttribute,"Always on top"))) {
  CFRetain(node);return node;
 }
 CFTypeRef children=NULL;
 if (AXUIElementCopyAttributeValue(node,kAXChildrenAttribute,&children)!=kAXErrorSuccess || !children) return NULL;
 AXUIElementRef found=NULL;
 if (CFGetTypeID(children)==CFArrayGetTypeID()) {
  for (CFIndex i=0;i<CFArrayGetCount(children) && !found;i++) found=adm_find_top_switch(CFArrayGetValueAtIndex(children,i),depth+1,budget);
 }
 CFRelease(children);return found;
}

// Return 0 when the switch reflects the requested state, 1 while Qt is
// delivering the update, 2 when the settings pane is not exposed yet.
static int adm_apply_emulator_top(int pid, int enabled) {
 AXUIElementRef app=AXUIElementCreateApplication(pid);
 int budget=2048;
 AXUIElementRef checkbox=adm_find_top_switch(app,0,&budget);
 CFRelease(app);
 if (!checkbox) return 2;
 CFTypeRef value=NULL;
 int current=-1;
 if (AXUIElementCopyAttributeValue(checkbox,kAXValueAttribute,&value)==kAXErrorSuccess && value) {
  if (CFGetTypeID(value)==CFNumberGetTypeID()) CFNumberGetValue(value,kCFNumberIntType,&current);
  else if (CFGetTypeID(value)==CFBooleanGetTypeID()) current=CFBooleanGetValue(value);
  CFRelease(value);
 }
 if (current!=0 && current!=1) {CFRelease(checkbox);return 3;}
 if (current==enabled) {CFRelease(checkbox);return 0;}
 AXError error=AXUIElementPerformAction(checkbox,kAXPressAction);
 CFRelease(checkbox);
 return error==kAXErrorSuccess ? 1 : 4;
}
*/
import "C"

import (
	"fmt"
	"time"
	"unsafe"
)

func emulatorWindowTopNative(pid int, avd string) (bool, error) {
	name := C.CString(avd)
	defer C.free(unsafe.Pointer(name))
	value := int(C.adm_emulator_window_top(C.int(pid), name))
	if value < 0 {
		return false, fmt.Errorf("未找到模拟器可见窗口")
	}
	return value == 1, nil
}

func applyEmulatorWindowTopNative(pid int, avd string, enabled bool) error {
	requested := C.int(0)
	if enabled {
		requested = 1
	}
	last := 2
	for attempt := 0; attempt < 30; attempt++ {
		last = int(C.adm_apply_emulator_top(C.int(pid), requested))
		if last == 0 {
			top, err := emulatorWindowTopNative(pid, avd)
			if err == nil && top == enabled {
				return nil
			}
		}
		if last == 3 || last == 4 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("模拟器始终置顶未生效：无法确认原生设置及窗口层级（状态 %d）", last)
}
