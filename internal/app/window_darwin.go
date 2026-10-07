//go:build darwin

package app

/*
#cgo darwin LDFLAGS: -framework ApplicationServices -framework CoreGraphics -framework CoreFoundation -framework AppKit -lobjc
#include <ApplicationServices/ApplicationServices.h>
#include <CoreGraphics/CoreGraphics.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <math.h>
#include <objc/runtime.h>
#include <objc/message.h>

static int adm_activate_process(int pid) {
 id cls = (id)objc_getClass("NSRunningApplication");
 id app = ((id (*)(id, SEL, int))objc_msgSend)(cls, sel_registerName("runningApplicationWithProcessIdentifier:"), pid);
 if (app == NULL) { return 0; }
 return ((BOOL (*)(id, SEL, unsigned long))objc_msgSend)(app, sel_registerName("activateWithOptions:"), 2) != 0;
}

static char* adm_strdup(const char* value) {
	if (value == NULL) {
		return strdup("");
	}
	return strdup(value);
}

static int cfstring_has_prefix(CFStringRef value, const char* prefix) {
	if (value == NULL || prefix == NULL) {
		return 0;
	}
	char buffer[256];
	if (!CFStringGetCString(value, buffer, sizeof(buffer), kCFStringEncodingUTF8)) {
		return 0;
	}
	return strncmp(buffer, prefix, strlen(prefix)) == 0;
}

static int cfstring_contains(CFStringRef value, const char* needle) {
	if (value == NULL || needle == NULL || needle[0] == '\0') {
		return 0;
	}
	CFStringRef needleRef = CFStringCreateWithCString(NULL, needle, kCFStringEncodingUTF8);
	if (needleRef == NULL) {
		return 0;
	}
	CFRange found = CFStringFind(value, needleRef, 0);
	CFRelease(needleRef);
	return found.location != kCFNotFound;
}

static const char* adm_serial_port(const char* serial) {
	if (serial == NULL) {
		return "";
	}
	if (strncmp(serial, "emulator-", 9) == 0) {
		return serial + 9;
	}
	return serial;
}

static int title_matches(CFStringRef title, const char* avd, const char* serial) {
	if (title == NULL) {
		return 0;
	}
	if (avd != NULL && avd[0] != '\0' && cfstring_contains(title, avd)) {
		return 1;
	}
	const char* port = adm_serial_port(serial);
	if (port != NULL && port[0] != '\0') {
		char needle[64];
		snprintf(needle, sizeof(needle), ":%s", port);
		if (cfstring_contains(title, needle)) {
			return 1;
		}
	}
	return 0;
}

static AXUIElementRef copy_matching_ax_window(pid_t pid, CFStringRef cgTitle, const char* avd, const char* serial) {
	AXUIElementRef app = AXUIElementCreateApplication(pid);
	if (app == NULL) {
		return NULL;
	}
	CFTypeRef windowsValue = NULL;
	if (AXUIElementCopyAttributeValue(app, kAXWindowsAttribute, &windowsValue) != kAXErrorSuccess || windowsValue == NULL) {
		CFRelease(app);
		return NULL;
	}
	CFArrayRef windows = (CFArrayRef)windowsValue;
	CFIndex count = CFArrayGetCount(windows);
	AXUIElementRef largestWindow = NULL;
	double largestArea = 0.0;
	for (CFIndex i = 0; i < count; i++) {
		AXUIElementRef win = (AXUIElementRef)CFArrayGetValueAtIndex(windows, i);
		CFTypeRef titleValue = NULL;
		CFStringRef axTitle = NULL;
		if (AXUIElementCopyAttributeValue(win, kAXTitleAttribute, &titleValue) == kAXErrorSuccess && titleValue != NULL) {
			axTitle = (CFStringRef)titleValue;
		}
		int matched = 0;
		if (axTitle != NULL && title_matches(axTitle, avd, serial)) {
			matched = 1;
		} else if (axTitle != NULL && cgTitle != NULL && CFStringCompare(axTitle, cgTitle, 0) == kCFCompareEqualTo) {
			matched = 1;
		}
		if (titleValue != NULL) {
			CFRelease(titleValue);
		}
		if (matched) {
			CFRetain(win);
			CFRelease(windowsValue);
			CFRelease(app);
			if (largestWindow != NULL) {
				CFRelease(largestWindow);
			}
			return win;
		}
		CFTypeRef sizeValue = NULL;
		if (AXUIElementCopyAttributeValue(win, kAXSizeAttribute, &sizeValue) == kAXErrorSuccess && sizeValue != NULL) {
			CGSize size = CGSizeZero;
			if (AXValueGetValue((AXValueRef)sizeValue, kAXValueCGSizeType, &size)) {
				double area = (double)size.width * (double)size.height;
				if (area > largestArea) {
					if (largestWindow != NULL) {
						CFRelease(largestWindow);
					}
					CFRetain(win);
					largestWindow = win;
					largestArea = area;
				}
			}
			CFRelease(sizeValue);
		}
	}
	CFRelease(windowsValue);
	CFRelease(app);
	return largestWindow;
}

static AXUIElementRef copy_window_for_target(const char* avd, const char* serial) {
	CFArrayRef infos = CGWindowListCopyWindowInfo(kCGWindowListOptionAll, kCGNullWindowID);
	if (infos == NULL) {
		return NULL;
	}
	CFIndex count = CFArrayGetCount(infos);
	for (CFIndex i = 0; i < count; i++) {
		CFDictionaryRef info = (CFDictionaryRef)CFArrayGetValueAtIndex(infos, i);
		CFStringRef owner = (CFStringRef)CFDictionaryGetValue(info, kCGWindowOwnerName);
		if (!cfstring_has_prefix(owner, "qemu-system")) {
			continue;
		}
		CFStringRef title = (CFStringRef)CFDictionaryGetValue(info, kCGWindowName);
		if (!title_matches(title, avd, serial)) {
			continue;
		}
		CFNumberRef pidNumber = (CFNumberRef)CFDictionaryGetValue(info, kCGWindowOwnerPID);
		pid_t pid = 0;
		if (pidNumber == NULL || !CFNumberGetValue(pidNumber, kCFNumberSInt32Type, &pid)) {
			continue;
		}
		AXUIElementRef win = copy_matching_ax_window(pid, title, avd, serial);
		if (win != NULL) {
			CFRelease(infos);
			return win;
		}
	}
	CFRelease(infos);
	return NULL;
}

static AXUIElementRef copy_window_for_pid(pid_t pid) {
	if (pid <= 0) {
		return NULL;
	}
	return copy_matching_ax_window(pid, NULL, "", "");
}

static int copy_ax_window_frame(AXUIElementRef win, CGRect* frame) {
	if (win == NULL || frame == NULL) {
		return 0;
	}
	CFTypeRef positionValue = NULL;
	CFTypeRef sizeValue = NULL;
	CGPoint position = CGPointZero;
	CGSize size = CGSizeZero;
	int ok = 0;
	if (AXUIElementCopyAttributeValue(win, kAXPositionAttribute, &positionValue) == kAXErrorSuccess &&
		AXUIElementCopyAttributeValue(win, kAXSizeAttribute, &sizeValue) == kAXErrorSuccess &&
		positionValue != NULL && sizeValue != NULL &&
		AXValueGetValue((AXValueRef)positionValue, kAXValueCGPointType, &position) &&
		AXValueGetValue((AXValueRef)sizeValue, kAXValueCGSizeType, &size)) {
		*frame = CGRectMake(position.x, position.y, size.width, size.height);
		ok = 1;
	}
	if (positionValue != NULL) {
		CFRelease(positionValue);
	}
	if (sizeValue != NULL) {
		CFRelease(sizeValue);
	}
	return ok;
}

static AXUIElementRef copy_toolbar_for_main_window(AXUIElementRef mainWindow) {
	if (mainWindow == NULL) {
		return NULL;
	}
	CFTypeRef parentValue = NULL;
	if (AXUIElementCopyAttributeValue(mainWindow, kAXParentAttribute, &parentValue) != kAXErrorSuccess || parentValue == NULL) {
		return NULL;
	}
	AXUIElementRef app = (AXUIElementRef)parentValue;
	CFTypeRef windowsValue = NULL;
	if (AXUIElementCopyAttributeValue(app, kAXWindowsAttribute, &windowsValue) != kAXErrorSuccess || windowsValue == NULL) {
		CFRelease(parentValue);
		return NULL;
	}
	CFArrayRef windows = (CFArrayRef)windowsValue;
	CFIndex count = CFArrayGetCount(windows);
	AXUIElementRef toolbar = NULL;
	double bestArea = 0.0;
	for (CFIndex i = 0; i < count; i++) {
		AXUIElementRef candidate = (AXUIElementRef)CFArrayGetValueAtIndex(windows, i);
		if (candidate == NULL || CFEqual(candidate, mainWindow)) {
			continue;
		}
		CFTypeRef titleValue = NULL;
		int hasTitle = 0;
		if (AXUIElementCopyAttributeValue(candidate, kAXTitleAttribute, &titleValue) == kAXErrorSuccess && titleValue != NULL) {
			if (CFGetTypeID(titleValue) == CFStringGetTypeID() && CFStringGetLength((CFStringRef)titleValue) > 0) {
				hasTitle = 1;
			}
			CFRelease(titleValue);
		}
		if (hasTitle) {
			continue;
		}
		CGRect frame = CGRectZero;
		if (!copy_ax_window_frame(candidate, &frame)) {
			continue;
		}
		if (frame.size.width < 40.0 || frame.size.width > 120.0 || frame.size.height < 250.0) {
			continue;
		}
		double area = (double)frame.size.width * (double)frame.size.height;
		if (area > bestArea) {
			if (toolbar != NULL) {
				CFRelease(toolbar);
			}
			CFRetain(candidate);
			toolbar = candidate;
			bestArea = area;
		}
	}
	CFRelease(windowsValue);
	CFRelease(parentValue);
	return toolbar;
}

static int copy_current_app_largest_window_frame(CGRect* frame) {
	if (frame == NULL) {
		return 0;
	}
	AXUIElementRef app = AXUIElementCreateApplication(getpid());
	if (app == NULL) {
		return 0;
	}
	CFTypeRef windowsValue = NULL;
	if (AXUIElementCopyAttributeValue(app, kAXWindowsAttribute, &windowsValue) != kAXErrorSuccess || windowsValue == NULL) {
		CFRelease(app);
		return 0;
	}
	CFArrayRef windows = (CFArrayRef)windowsValue;
	CFIndex count = CFArrayGetCount(windows);
	double largestArea = 0.0;
	int found = 0;
	for (CFIndex i = 0; i < count; i++) {
		AXUIElementRef win = (AXUIElementRef)CFArrayGetValueAtIndex(windows, i);
		CGRect candidate = CGRectZero;
		if (!copy_ax_window_frame(win, &candidate)) {
			continue;
		}
		double area = (double)candidate.size.width * (double)candidate.size.height;
		if (area > largestArea) {
			*frame = candidate;
			largestArea = area;
			found = 1;
		}
	}
	CFRelease(windowsValue);
	CFRelease(app);
	return found;
}

static CGRect adm_device_wall_work_area(CGRect bounds, CGRect appFrame, int cols) {
	CGFloat margin = 6.0;
	CGFloat leftInset = 16.0;
	CGFloat rightInset = 16.0;
	CGFloat topInset = 100.0;
	CGFloat bottomInset = 56.0;
	CGRect area = CGRectMake(
		appFrame.origin.x + leftInset,
		appFrame.origin.y + topInset,
		appFrame.size.width - leftInset - rightInset,
		appFrame.size.height - topInset - bottomInset
	);
	CGFloat minUsefulW = 288.0 * cols + margin * (cols - 1);
	if (area.size.width >= minUsefulW && area.size.height >= 360.0) {
		return area;
	}
	return CGRectMake(
		bounds.origin.x + margin,
		bounds.origin.y + 42.0,
		bounds.size.width - margin * 2.0,
		bounds.size.height - 42.0 - margin
	);
}


static void activate_ax_window(AXUIElementRef win) {
	if (win == NULL) {
		return;
	}
	CFTypeRef parent = NULL;
	if (AXUIElementCopyAttributeValue(win, kAXParentAttribute, &parent) == kAXErrorSuccess && parent != NULL) {
		AXUIElementSetAttributeValue((AXUIElementRef)parent, kAXFrontmostAttribute, kCFBooleanTrue);
		CFRelease(parent);
	}
	AXUIElementSetAttributeValue(win, kAXMinimizedAttribute, kCFBooleanFalse);
	AXUIElementSetAttributeValue(win, kAXMainAttribute, kCFBooleanTrue);
	AXUIElementSetAttributeValue(win, kAXFocusedAttribute, kCFBooleanTrue);
	AXUIElementPerformAction(win, kAXRaiseAction);
}

static int adm_ax_trust_state(void) {
	if (AXIsProcessTrusted()) {
		return 1;
	}
	return 0;
}

char* adm_ax_focus_window(const char* avd, const char* serial) {
	int trustState = adm_ax_trust_state();
	if (trustState == 2) {
		return adm_strdup("permission-prompted");
	}
	if (trustState != 1) {
		return adm_strdup("permission");
	}
	AXUIElementRef win = copy_window_for_target(avd, serial);
	if (win == NULL) {
		return adm_strdup("not-found");
	}
	activate_ax_window(win);
	AXError raiseErr = AXUIElementPerformAction(win, kAXRaiseAction);
	CFRelease(win);
	if (raiseErr != kAXErrorSuccess) {
		return adm_strdup("raise-failed");
	}
	return adm_strdup("ok");
}

char* adm_ax_focus_window_for_pid(int pid) {
	int trustState = adm_ax_trust_state();
	if (trustState == 2) {
		return adm_strdup("permission-prompted");
	}
	if (trustState != 1) {
		return adm_strdup("permission");
	}
	AXUIElementRef win = copy_window_for_pid((pid_t)pid);
	if (win == NULL) {
		return adm_strdup("not-found");
	}
	activate_ax_window(win);
	AXError raiseErr = AXUIElementPerformAction(win, kAXRaiseAction);
	CFRelease(win);
	if (raiseErr != kAXErrorSuccess) {
		return adm_strdup("raise-failed");
	}
	return adm_strdup("ok");
}

// AX updates are delivered to another process asynchronously. A successful
// setter alone does not prove that process accepted the requested outer size.
static int set_ax_window_size_verified(AXUIElementRef win, CGSize requested, CGRect* actual, AXError* lastError) {
 *actual = CGRectZero;
 copy_ax_window_frame(win,actual);
 *lastError = kAXErrorSuccess;
 if (requested.width<1.0 || requested.height<1.0) {*lastError=kAXErrorIllegalArgument;return 1;}
 for (int attempt = 0; attempt < 3; attempt++) {
  AXValueRef value = AXValueCreate(kAXValueCGSizeType, &requested);
  if (value == NULL) { return 1; }
  *lastError = AXUIElementSetAttributeValue(win, kAXSizeAttribute, value);
  CFRelease(value);
  if (*lastError != kAXErrorSuccess) { return 1; }
  int matching = 0;
  for (int poll = 0; poll < 12; poll++) {
   usleep(30000);
   if (copy_ax_window_frame(win, actual)) {
    if (fabs(actual->size.width - requested.width) <= 2.0 && fabs(actual->size.height - requested.height) <= 2.0) {
     matching++;
     if (matching >= 2) { return 0; }
    } else { matching = 0; }
   }
  }
 }
 return actual->size.width > 0.0 ? 2 : 3;
}

static int set_ax_window_position_verified(AXUIElementRef win, CGPoint requested, CGRect* actual, AXError* lastError) {
 *lastError = kAXErrorSuccess;
 for (int attempt = 0; attempt < 3; attempt++) {
  AXValueRef value = AXValueCreate(kAXValueCGPointType, &requested);
  if (value == NULL) { return 1; }
  *lastError = AXUIElementSetAttributeValue(win, kAXPositionAttribute, value);
  CFRelease(value);
  if (*lastError != kAXErrorSuccess) { return 1; }
  int matching = 0;
  for (int poll = 0; poll < 8; poll++) {
   usleep(20000);
   if (copy_ax_window_frame(win, actual)) {
    if (fabs(actual->origin.x-requested.x)<=2.0 && fabs(actual->origin.y-requested.y)<=2.0) {
     matching++;
     if (matching>=2) { return 0; }
    } else { matching=0; }
   }
  }
 }
 return 2;
}

static CGSize adm_uniform_tile_size(double availableW, double availableH, int cols, int rows, double auxiliaryW, double nominalW) {
 if (cols<1) {cols=1;} if (rows<1) {rows=1;}
 double maxW=availableW/cols-6.0-auxiliaryW;
 double maxH=availableH/rows-6.0;
 double scale=fmin(1.0,fmin(maxW/nominalW,maxH/624.0));
 if (scale<=0.0) { return CGSizeZero; }
 return CGSizeMake(floor(nominalW*scale), floor(624.0*scale));
}

char* adm_ax_read_window_frame_for_pid(int pid, double* x, double* y, double* width, double* height) {
 if (!AXIsProcessTrusted()) {return adm_strdup("permission");}
 AXUIElementRef win=copy_window_for_pid((pid_t)pid);
 if (win==NULL) {return adm_strdup("not-found");}
 CGRect frame=CGRectZero;
 int found=copy_ax_window_frame(win,&frame);
 CFRelease(win);
 if (!found) {return adm_strdup("frame-unavailable");}
 *x=frame.origin.x;*y=frame.origin.y;*width=frame.size.width;*height=frame.size.height;
 return adm_strdup("ok");
}

char* adm_ax_resize_window_for_pid(int pid, double width, double height) {
	int trustState = adm_ax_trust_state();
	if (trustState == 2) {
		return adm_strdup("permission-prompted");
	}
	if (trustState != 1) {
		return adm_strdup("permission");
	}
	if (width < 0.0 || height <= 0.0) {
		return adm_strdup("invalid-size");
	}
	AXUIElementRef win = copy_window_for_pid((pid_t)pid);
	if (win == NULL) {
		return adm_strdup("not-found");
	}
 // scrcpy uses a 30-point macOS title bar. Width zero requests a
 // common outer height with the device's current content aspect preserved.
 if (width == 0.0) {
  CGRect frame=CGRectZero;
  if (!copy_ax_window_frame(win,&frame) || frame.size.height<=30.0) {
   CFRelease(win);return adm_strdup("frame-unavailable");
  }
  width=round(frame.size.width*(height-30.0)/(frame.size.height-30.0));
 }
 CGSize size=CGSizeMake(width,height);
 activate_ax_window(win);
 CGRect actual=CGRectZero;AXError sizeErr=kAXErrorSuccess;
 int resized=set_ax_window_size_verified(win,size,&actual,&sizeErr);
 AXUIElementPerformAction(win,kAXRaiseAction);
 CFRelease(win);
 if (resized!=0) {
  char message[256];
  snprintf(message,sizeof(message),"resize-unverified requested=%.0fx%.0f actual=%.0fx%.0f ax=%d",width,height,actual.size.width,actual.size.height,(int)sizeErr);
  return adm_strdup(message);
 }
 return adm_strdup("ok");
}

static char* tile_ax_windows(AXUIElementRef* windows, int matched, int requestedCols) {
	if (matched < 1) {
		return adm_strdup("not-found");
	}

	CGRect bounds = CGDisplayBounds(CGMainDisplayID());
	CGFloat margin = 6.0;
	CGFloat horizontalGap = 6.0;
	CGFloat targetWindowW = 288.0;
	CGFloat targetWindowH = 624.0;
	CGFloat visibleWindowW = 288.0;
	// Android Emulator exposes the side toolbar as a sibling AX window.
	CGFloat sideChromeW = 61.0;
	CGFloat maxToolbarW = 0.0;
 CGFloat maxNaturalWidth=0.0;
 int* hasToolbar = calloc((size_t)matched, sizeof(int));
	if (hasToolbar == NULL) {
		for (int i = 0; i < matched; i++) {
			CFRelease(windows[i]);
		}
		return adm_strdup("alloc-failed");
	}
	for (int i = 0; i < matched; i++) {
		AXUIElementRef toolbar = copy_toolbar_for_main_window(windows[i]);
		if (toolbar != NULL) {
			hasToolbar[i] = 1;
 maxToolbarW=sideChromeW;
			CFRelease(toolbar);
		}
  CGRect initial=CGRectZero;
  if (copy_ax_window_frame(windows[i],&initial) && initial.size.height>30.0) {
   CGFloat chrome=hasToolbar[i]?0.0:30.0;
   CGFloat naturalWidth=initial.size.width*(624.0-chrome)/(initial.size.height-chrome);
   maxNaturalWidth=fmax(maxNaturalWidth,naturalWidth);
  }
	}
 if (maxNaturalWidth>0.0) {visibleWindowW=maxNaturalWidth;}
	CGRect appFrame = CGRectZero;
	CGRect workArea = CGRectMake(
		bounds.origin.x + margin,
		bounds.origin.y + 42.0,
		bounds.size.width - margin * 2.0,
		bounds.size.height - 42.0 - margin
	);
	if (copy_current_app_largest_window_frame(&appFrame)) {
		CGRect deviceWallArea = adm_device_wall_work_area(bounds, appFrame, requestedCols > 0 ? requestedCols : matched);
		CGFloat fullRowW = 0.0;
		for (int i = 0; i < matched; i++) {
			if (i > 0) {
				fullRowW += horizontalGap;
			}
			fullRowW += visibleWindowW;
			if (hasToolbar[i]) {
				fullRowW += sideChromeW;
			}
		}
		if (fullRowW <= deviceWallArea.size.width) {
			workArea = deviceWallArea;
		} else if (fullRowW <= bounds.size.width - margin * 2.0) {
			workArea = CGRectMake(
				bounds.origin.x + margin,
				deviceWallArea.origin.y,
				bounds.size.width - margin * 2.0,
				deviceWallArea.size.height
			);
		} else {
			workArea = deviceWallArea;
		}
	}

	int cols = requestedCols;
	if (cols < 1) {
		cols = 1;
		CGFloat rowW = 0.0;
		for (int i = 0; i < matched; i++) {
			CGFloat itemW = visibleWindowW;
			if (hasToolbar[i]) {
				itemW += sideChromeW;
			}
			CGFloat nextW = itemW;
			if (i > 0) {
				nextW += horizontalGap;
			}
			if (i > 0 && rowW + nextW > workArea.size.width) {
				break;
			}
			rowW += nextW;
			cols = i + 1;
		}
	}
	if (cols < 1) {
		cols = 1;
	}
	if (cols > matched) {
		cols = matched;
	}
	int rows = matched / cols;
	if (matched % cols != 0) {
		rows++;
	}
	CGFloat usableW = workArea.size.width;
	CGFloat usableH = workArea.size.height;
	CGFloat cellH = usableH / rows;

 CGSize uniformSize=adm_uniform_tile_size(usableW,usableH,cols,rows,maxToolbarW,visibleWindowW);
 targetWindowW=uniformSize.width;targetWindowH=uniformSize.height;
 char diagnostics[512]="";
	CGFloat* rowNextX = calloc((size_t)matched, sizeof(CGFloat));
	CGFloat* rowY = calloc((size_t)matched, sizeof(CGFloat));
	CGFloat* rowHeights = calloc((size_t)matched, sizeof(CGFloat));
	CGFloat* itemWidths = calloc((size_t)matched, sizeof(CGFloat));
	CGFloat* itemHeights = calloc((size_t)matched, sizeof(CGFloat));
	CGRect* mainFrames = calloc((size_t)matched, sizeof(CGRect));
	AXUIElementRef* toolbars = calloc((size_t)matched, sizeof(AXUIElementRef));
	CGFloat* toolbarWidths = calloc((size_t)matched, sizeof(CGFloat));
	if (rowNextX == NULL || rowY == NULL || rowHeights == NULL || itemWidths == NULL || itemHeights == NULL || mainFrames == NULL || toolbars == NULL || toolbarWidths == NULL) {
		free(rowNextX);
		free(rowY);
		free(rowHeights);
		free(itemWidths);
		free(itemHeights);
		free(mainFrames);
		free(toolbars);
		free(toolbarWidths);
		free(hasToolbar);
		for (int i = 0; i < matched; i++) {
			CFRelease(windows[i]);
		}
		return adm_strdup("alloc-failed");
	}
	for (int row = 0; row < rows; row++) {
		rowNextX[row] = workArea.origin.x;
	}

	for (int i = 0; i < matched; i++) {
		int row = i / cols;
 CGFloat windowW=targetWindowW;CGFloat windowH=targetWindowH;
  CGRect original=CGRectZero;
  CGFloat chrome=hasToolbar[i]?0.0:30.0;
  if (copy_ax_window_frame(windows[i],&original) && original.size.height>chrome) {
   windowW=round(original.size.width*(windowH-chrome)/(original.size.height-chrome));
  }
  CGFloat cellX=rowNextX[row];CGFloat cellY=workArea.origin.y+row*cellH;
  CGPoint position=CGPointMake(cellX,cellY);CGSize size=CGSizeMake(windowW,windowH);
  activate_ax_window(windows[i]);
  CGRect actual=CGRectZero;AXError sizeError=kAXErrorSuccess;
  int resized=set_ax_window_size_verified(windows[i],size,&actual,&sizeError);
  if (resized!=0 && diagnostics[0]=='\0') {
   snprintf(diagnostics,sizeof(diagnostics),"resize-unverified index=%d requested=%.0fx%.0f actual=%.0fx%.0f ax=%d",i,windowW,windowH,actual.size.width,actual.size.height,(int)sizeError);
  }
  AXError positionError=kAXErrorSuccess;
  if (set_ax_window_position_verified(windows[i],position,&actual,&positionError)!=0 && diagnostics[0]=='\0') {
   snprintf(diagnostics,sizeof(diagnostics),"position-unverified index=%d ax=%d",i,(int)positionError);
  }

		AXUIElementRef toolbar = copy_toolbar_for_main_window(windows[i]);
		if (toolbar != NULL) {
			toolbarWidths[i] = sideChromeW;
			CGPoint toolbarPosition = CGPointMake(
				cellX + actual.size.width,
				cellY + 28.0
			);
			AXValueRef toolbarPositionValue = AXValueCreate(kAXValueCGPointType, &toolbarPosition);
			if (toolbarPositionValue != NULL) {
				AXUIElementSetAttributeValue(toolbar, kAXPositionAttribute, toolbarPositionValue);
				CFRelease(toolbarPositionValue);
			}
			toolbars[i] = toolbar;
		} else {
			toolbarWidths[i] = 0.0;
		}
		CGRect actualFrame = CGRectZero;
		if (copy_ax_window_frame(windows[i], &actualFrame)) {
			mainFrames[i] = actualFrame;
		} else {
			mainFrames[i] = CGRectMake(cellX, cellY, windowW, windowH);
		}
		CGFloat finalMainW = visibleWindowW;
		if (mainFrames[i].size.width > 1.0) {
			finalMainW = mainFrames[i].size.width;
		}
		itemWidths[i] = finalMainW + toolbarWidths[i];
		itemHeights[i] = mainFrames[i].size.height;
		rowNextX[row] = cellX + finalMainW + toolbarWidths[i] + horizontalGap;
	}

	int finalCols = cols;
	if (requestedCols < 1) {
		finalCols = 1;
		CGFloat rowW = 0.0;
		for (int i = 0; i < matched; i++) {
			CGFloat nextW = itemWidths[i];
			if (i > 0) {
				nextW += horizontalGap;
			}
			if (i > 0 && rowW + nextW > workArea.size.width) {
				break;
			}
			rowW += nextW;
			finalCols = i + 1;
		}
	}
	if (finalCols < 1) {
		finalCols = 1;
	}
	if (finalCols > matched) {
		finalCols = matched;
	}
	int finalRows = matched / finalCols;
	if (matched % finalCols != 0) {
		finalRows++;
	}
	for (int row = 0; row < finalRows; row++) {
		rowNextX[row] = workArea.origin.x;
	}
	for (int i = 0; i < matched; i++) {
		int row = i / finalCols;
		if (itemHeights[i] > rowHeights[row]) {
			rowHeights[row] = itemHeights[i];
		}
	}
	rowY[0] = workArea.origin.y;
	for (int row = 1; row < finalRows; row++) {
		rowY[row] = rowY[row - 1] + rowHeights[row - 1] + margin;
	}
	for (int i = 0; i < matched; i++) {
		int row = i / finalCols;
		CGFloat finalX = rowNextX[row];
		CGFloat finalY = rowY[row];
		CGFloat finalMainW = itemWidths[i] - toolbarWidths[i];
		CGPoint finalPosition = CGPointMake(finalX, finalY);
 CGRect positionedFrame=CGRectZero;AXError positionError=kAXErrorSuccess;
  if (set_ax_window_position_verified(windows[i],finalPosition,&positionedFrame,&positionError)!=0 && diagnostics[0]=='\0') {
   snprintf(diagnostics,sizeof(diagnostics),"position-unverified index=%d ax=%d",i,(int)positionError);
  }

		CGRect finalFrame = CGRectZero;
		if (copy_ax_window_frame(windows[i], &finalFrame) && finalFrame.size.width > 1.0) {
			finalX = finalFrame.origin.x;
			finalY = finalFrame.origin.y;
			finalMainW = finalFrame.size.width;
   if ((fabs(finalFrame.size.width-mainFrames[i].size.width)>2.0 || fabs(finalFrame.size.height-targetWindowH)>2.0) && diagnostics[0]=='\0') {
    snprintf(diagnostics,sizeof(diagnostics),"resize-unverified index=%d requested=%.0fx%.0f actual=%.0fx%.0f",i,mainFrames[i].size.width,targetWindowH,finalFrame.size.width,finalFrame.size.height);
   }
		}
		if (toolbars[i] != NULL) {
			CGPoint finalToolbarPosition = CGPointMake(
				finalX + finalMainW,
				finalY + 28.0
			);
 CGRect toolbarFrame=CGRectZero;AXError toolbarError=kAXErrorSuccess;
   if (set_ax_window_position_verified(toolbars[i],finalToolbarPosition,&toolbarFrame,&toolbarError)!=0 && diagnostics[0]=='\0') {
    snprintf(diagnostics,sizeof(diagnostics),"toolbar-position-unverified index=%d ax=%d",i,(int)toolbarError);
   }
		}
		rowNextX[row] = finalX + finalMainW + toolbarWidths[i] + horizontalGap;
	}

	for (int i = 0; i < matched; i++) {
		if (toolbars[i] != NULL) {
			AXUIElementPerformAction(toolbars[i], kAXRaiseAction);
			CFRelease(toolbars[i]);
		}
		CFRelease(windows[i]);
	}
	free(rowNextX);
	free(rowY);
	free(rowHeights);
	free(itemWidths);
	free(itemHeights);
	free(mainFrames);
	free(toolbars);
	free(toolbarWidths);
	free(hasToolbar);
 return adm_strdup(diagnostics[0] ? diagnostics : "ok");
}

char* adm_ax_tile_windows(const char** avds, const char** serials, int count, int requestedCols) {
	int trustState = adm_ax_trust_state();
	if (trustState == 2) {
		return adm_strdup("permission-prompted");
	}
	if (trustState != 1) {
		return adm_strdup("permission");
	}
	if (count < 1) {
		return adm_strdup("no-targets");
	}
	AXUIElementRef* windows = calloc((size_t)count, sizeof(AXUIElementRef));
	if (windows == NULL) {
		return adm_strdup("alloc-failed");
	}
	int matched = 0;
	for (int i = 0; i < count; i++) {
		AXUIElementRef win = copy_window_for_target(avds[i], serials[i]);
		if (win != NULL) {
			windows[matched++] = win;
		}
	}
	if (matched < 1) {
		free(windows);
		return adm_strdup("not-found");
	}

	char* status = tile_ax_windows(windows, matched, requestedCols);
	free(windows);
	return status;
}

char* adm_ax_tile_windows_for_pids(const int* pids, int count, int requestedCols) {
	int trustState = adm_ax_trust_state();
	if (trustState == 2) {
		return adm_strdup("permission-prompted");
	}
	if (trustState != 1) {
		return adm_strdup("permission");
	}
	if (count < 1) {
		return adm_strdup("no-targets");
	}
	AXUIElementRef* windows = calloc((size_t)count, sizeof(AXUIElementRef));
	if (windows == NULL) {
		return adm_strdup("alloc-failed");
	}
	int matched = 0;
	for (int i = 0; i < count; i++) {
		AXUIElementRef win = copy_window_for_pid((pid_t)pids[i]);
		if (win != NULL) {
			windows[matched++] = win;
		}
	}
	if (matched < 1) {
		free(windows);
		return adm_strdup("not-found");
	}
	char* status = tile_ax_windows(windows, matched, requestedCols);
	free(windows);
	return status;
}
*/
import "C"

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"unsafe"
)

func MainDisplaySize() (int, int, bool) {
	bounds := C.CGDisplayBounds(C.CGMainDisplayID())
	width := int(bounds.size.width)
	height := int(bounds.size.height)
	return width, height, width > 0 && height > 0
}

func activateProcessNative(pid int) error {
	if C.adm_activate_process(C.int(pid)) == 0 {
		return fmt.Errorf("无法激活外部窗口进程：%d", pid)
	}
	return nil
}

func accessibilityTrustedNative() bool {
	return C.AXIsProcessTrusted() != 0
}

func focusEmulatorWindowNative(avdName, serial string) error {
	cAVD := C.CString(avdName)
	cSerial := C.CString(serial)
	defer C.free(unsafe.Pointer(cAVD))
	defer C.free(unsafe.Pointer(cSerial))
	result := C.adm_ax_focus_window(cAVD, cSerial)
	defer C.free(unsafe.Pointer(result))
	switch C.GoString(result) {
	case "ok":
		return nil
	case "permission-prompted":
		return &AccessibilityPermissionPromptedError{Operation: "聚焦外部模拟器窗口"}
	case "permission":
		return &AccessibilityPermissionRequiredError{Operation: "聚焦外部模拟器窗口"}
	case "not-found":
		if pid, ok := qemuPIDForAVD(avdName); ok {
			return focusEmulatorWindowByPID(pid, avdName, serial)
		}
		return fmt.Errorf("未找到匹配窗口（AVD=%s, serial=%s）。模拟器进程存在性未确认，或窗口已关闭、隐藏、未暴露给 macOS", avdName, serial)
	default:
		return fmt.Errorf("聚焦模拟器窗口失败：%s", C.GoString(result))
	}
}

func focusEmulatorWindowByPID(pid int, avdName, serial string) error {
	result := C.adm_ax_focus_window_for_pid(C.int(pid))
	defer C.free(unsafe.Pointer(result))
	switch C.GoString(result) {
	case "ok":
		return nil
	case "permission-prompted":
		return &AccessibilityPermissionPromptedError{Operation: "聚焦外部模拟器窗口"}
	case "permission":
		return &AccessibilityPermissionRequiredError{Operation: "聚焦外部模拟器窗口"}
	case "not-found":
		return fmt.Errorf("已找到 qemu 进程 pid=%d（AVD=%s, serial=%s），但该进程没有向 macOS 暴露可操作窗口", pid, avdName, serial)
	default:
		return fmt.Errorf("聚焦模拟器窗口失败：%s", C.GoString(result))
	}
}

func focusProcessWindowNative(pid int) error {
	result := C.adm_ax_focus_window_for_pid(C.int(pid))
	defer C.free(unsafe.Pointer(result))
	switch C.GoString(result) {
	case "ok":
		return nil
	case "permission-prompted":
		return &AccessibilityPermissionPromptedError{Operation: "聚焦外部窗口"}
	case "permission":
		return &AccessibilityPermissionRequiredError{Operation: "聚焦外部窗口"}
	case "not-found":
		return fmt.Errorf("已找到进程 pid=%d，但该进程没有向 macOS 暴露可操作窗口", pid)
	default:
		return fmt.Errorf("聚焦外部窗口失败：%s", C.GoString(result))
	}
}

func resizeProcessWindowNative(pid int, width, height int) error {
	result := C.adm_ax_resize_window_for_pid(C.int(pid), C.double(width), C.double(height))
	defer C.free(unsafe.Pointer(result))
	switch C.GoString(result) {
	case "ok":
		return nil
	case "permission-prompted":
		return &AccessibilityPermissionPromptedError{Operation: "调整外部窗口尺寸"}
	case "permission":
		return &AccessibilityPermissionRequiredError{Operation: "调整外部窗口尺寸"}
	case "not-found":
		return fmt.Errorf("已找到进程 pid=%d，但该进程没有向 macOS 暴露可操作窗口", pid)
	case "invalid-size":
		return fmt.Errorf("无效的外部窗口尺寸：%dx%d", width, height)
	default:
		return fmt.Errorf("调整外部窗口尺寸失败：%s", C.GoString(result))
	}
}

func tileEmulatorWindowsNative(targets []emulatorWindowTarget, columns int) error {
	count := len(targets)
	if count == 0 {
		return fmt.Errorf("没有可平铺的模拟器窗口")
	}
	cAVDs := make([]*C.char, count)
	cSerials := make([]*C.char, count)
	for i, target := range targets {
		cAVDs[i] = C.CString(target.AVDName)
		cSerials[i] = C.CString(target.Serial)
		defer C.free(unsafe.Pointer(cAVDs[i]))
		defer C.free(unsafe.Pointer(cSerials[i]))
	}
	result := C.adm_ax_tile_windows((**C.char)(unsafe.Pointer(&cAVDs[0])), (**C.char)(unsafe.Pointer(&cSerials[0])), C.int(count), C.int(columns))
	defer C.free(unsafe.Pointer(result))
	switch C.GoString(result) {
	case "ok":
		return nil
	case "permission-prompted":
		return &AccessibilityPermissionPromptedError{Operation: "平铺外部模拟器窗口"}
	case "permission":
		return &AccessibilityPermissionRequiredError{Operation: "平铺外部模拟器窗口"}
	case "not-found":
		if err := tileEmulatorWindowsByPIDs(targets, columns); err == nil {
			return nil
		}
		return fmt.Errorf("没有找到可见的模拟器窗口。模拟器进程可能还在，但窗口已关闭、隐藏或未暴露给 macOS")
	default:
		return fmt.Errorf("平铺模拟器窗口失败：%s", C.GoString(result))
	}
}

func tileEmulatorWindowsByPIDs(targets []emulatorWindowTarget, columns int) error {
	pids := make([]C.int, 0, len(targets))
	seen := map[int]bool{}
	for _, target := range targets {
		pid, ok := qemuPIDForAVD(target.AVDName)
		if !ok || seen[pid] {
			continue
		}
		seen[pid] = true
		pids = append(pids, C.int(pid))
	}
	if len(pids) == 0 {
		return fmt.Errorf("没有从 qemu 命令行解析到目标 AVD 进程")
	}
	result := C.adm_ax_tile_windows_for_pids((*C.int)(unsafe.Pointer(&pids[0])), C.int(len(pids)), C.int(columns))
	defer C.free(unsafe.Pointer(result))
	switch C.GoString(result) {
	case "ok":
		return nil
	case "permission-prompted":
		return &AccessibilityPermissionPromptedError{Operation: "平铺外部模拟器窗口"}
	case "permission":
		return &AccessibilityPermissionRequiredError{Operation: "平铺外部模拟器窗口"}
	case "not-found":
		return fmt.Errorf("已找到 qemu 进程，但进程没有向 macOS 暴露可操作窗口")
	default:
		return fmt.Errorf("平铺模拟器窗口失败：%s", C.GoString(result))
	}
}

func tileProcessWindowsNative(pids []int, columns int) error {
	if len(pids) == 0 {
		return fmt.Errorf("没有可平铺的外部窗口进程")
	}
	cPIDs := make([]C.int, 0, len(pids))
	seen := map[int]bool{}
	for _, pid := range pids {
		if pid <= 0 || seen[pid] {
			continue
		}
		seen[pid] = true
		cPIDs = append(cPIDs, C.int(pid))
	}
	if len(cPIDs) == 0 {
		return fmt.Errorf("没有有效的外部窗口进程")
	}
	result := C.adm_ax_tile_windows_for_pids((*C.int)(unsafe.Pointer(&cPIDs[0])), C.int(len(cPIDs)), C.int(columns))
	defer C.free(unsafe.Pointer(result))
	switch C.GoString(result) {
	case "ok":
		return nil
	case "permission-prompted":
		return &AccessibilityPermissionPromptedError{Operation: "平铺外部窗口"}
	case "permission":
		return &AccessibilityPermissionRequiredError{Operation: "平铺外部窗口"}
	case "not-found":
		return fmt.Errorf("已找到进程，但没有找到可操作外部窗口")
	default:
		return fmt.Errorf("平铺外部窗口失败：%s", C.GoString(result))
	}
}

func qemuPIDForAVD(avdName string) (int, bool) {
	if strings.TrimSpace(avdName) == "" {
		return 0, false
	}
	output, err := exec.Command("ps", "-axo", "pid=,command=").Output()
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "qemu-system") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 || !commandLineHasAVD(fields[1:], avdName) {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err == nil && pid > 0 {
			return pid, true
		}
	}
	return 0, false
}

func commandLineHasAVD(fields []string, avdName string) bool {
	avdName = strings.TrimSpace(avdName)
	if avdName == "" {
		return false
	}
	atAVDName := "@" + avdName
	for _, field := range fields {
		if field == atAVDName {
			return true
		}
	}
	for i := 0; i < len(fields)-1; i++ {
		if fields[i] == "-avd" && fields[i+1] == avdName {
			return true
		}
	}
	return false
}

// ExternalWindowFrame is the actual AX outer frame in macOS display coordinates.
type ExternalWindowFrame struct{ X, Y, Width, Height float64 }

func ReadProcessWindowFrame(pid int) (ExternalWindowFrame, error) {
	var x, y, width, height C.double
	result := C.adm_ax_read_window_frame_for_pid(C.int(pid), &x, &y, &width, &height)
	defer C.free(unsafe.Pointer(result))
	status := C.GoString(result)
	if status == "permission" {
		return ExternalWindowFrame{}, &AccessibilityPermissionRequiredError{Operation: "读取外部窗口尺寸"}
	}
	if status != "ok" {
		return ExternalWindowFrame{}, fmt.Errorf("读取外部窗口尺寸失败（pid=%d）：%s", pid, status)
	}
	return ExternalWindowFrame{X: float64(x), Y: float64(y), Width: float64(width), Height: float64(height)}, nil
}

func uniformExternalTileSize(width, height float64, cols, rows int, auxiliaryWidth float64) (float64, float64) {
	size := C.adm_uniform_tile_size(C.double(width), C.double(height), C.int(cols), C.int(rows), C.double(auxiliaryWidth), C.double(288))
	return float64(size.width), float64(size.height)
}
