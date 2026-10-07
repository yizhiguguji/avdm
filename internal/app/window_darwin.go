//go:build darwin

package app

/*
#cgo darwin LDFLAGS: -framework ApplicationServices -framework CoreGraphics -framework CoreFoundation
#include <ApplicationServices/ApplicationServices.h>
#include <CoreGraphics/CoreGraphics.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

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
	CGFloat leftInset = 76.0;
	CGFloat rightInset = 350.0;
	CGFloat topInset = 122.0;
	CGFloat bottomInset = 112.0;
	CGRect area = CGRectMake(
		appFrame.origin.x + leftInset,
		appFrame.origin.y + topInset,
		appFrame.size.width - leftInset - rightInset,
		appFrame.size.height - topInset - bottomInset
	);
	CGFloat minUsefulW = 260.0 * cols + margin * (cols - 1);
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

static int adm_auto_tile_columns(int matched, CGRect workArea, CGFloat margin, CGFloat aspect, CGFloat targetW, CGFloat minW, CGFloat sideChromeW) {
	if (matched <= 1) {
		return 1;
	}
	int bestCols = 1;
	CGFloat bestScore = -1000000.0;
	int bestRows = matched;
	for (int candidate = 1; candidate <= matched; candidate++) {
		int rows = matched / candidate;
		if (matched % candidate != 0) {
			rows++;
		}
		CGFloat cellW = workArea.size.width / candidate;
		CGFloat cellH = workArea.size.height / rows;
		CGFloat mainWindowW = targetW;
		CGFloat combinedW = mainWindowW + sideChromeW;
		CGFloat maxCombinedW = cellW - margin;
		if (combinedW > maxCombinedW) {
			mainWindowW = maxCombinedW - sideChromeW;
			combinedW = maxCombinedW;
		}
		if (mainWindowW < 1.0) {
			mainWindowW = maxCombinedW;
			combinedW = mainWindowW;
		}
		CGFloat windowH = mainWindowW * aspect;
		CGFloat maxCellH = cellH - margin;
		if (windowH > maxCellH) {
			windowH = maxCellH;
			mainWindowW = windowH / aspect;
			combinedW = mainWindowW + sideChromeW;
		}
		if (mainWindowW < minW || windowH < 360.0) {
			continue;
		}
		CGFloat widthPenalty = targetW - mainWindowW;
		if (widthPenalty < 0) {
			widthPenalty = -widthPenalty;
		}
		CGFloat score = -widthPenalty - rows * 2.0 + candidate * 0.2;
		if (score > bestScore || (score == bestScore && rows < bestRows)) {
			bestScore = score;
			bestRows = rows;
			bestCols = candidate;
		}
	}
	return bestCols;
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

char* adm_ax_resize_window_for_pid(int pid, double width, double height) {
	int trustState = adm_ax_trust_state();
	if (trustState == 2) {
		return adm_strdup("permission-prompted");
	}
	if (trustState != 1) {
		return adm_strdup("permission");
	}
	if (width <= 0.0 || height <= 0.0) {
		return adm_strdup("invalid-size");
	}
	AXUIElementRef win = copy_window_for_pid((pid_t)pid);
	if (win == NULL) {
		return adm_strdup("not-found");
	}
	CGSize size = CGSizeMake(width, height);
	AXValueRef sizeValue = AXValueCreate(kAXValueCGSizeType, &size);
	if (sizeValue == NULL) {
		CFRelease(win);
		return adm_strdup("alloc-failed");
	}
	activate_ax_window(win);
	AXError sizeErr = AXUIElementSetAttributeValue(win, kAXSizeAttribute, sizeValue);
	CFRelease(sizeValue);
	AXUIElementPerformAction(win, kAXRaiseAction);
	CFRelease(win);
	if (sizeErr != kAXErrorSuccess) {
		return adm_strdup("resize-failed");
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
	CGFloat targetWindowW = 256.0;
	CGFloat targetWindowH = 600.0;
	CGFloat visibleWindowW = 256.0;
	// Android Emulator exposes the side toolbar as a sibling AX window.
	CGFloat sideChromeW = 61.0;
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
			CFRelease(toolbar);
		}
	}
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
	CGFloat cellW = usableW / cols;
	CGFloat cellH = usableH / rows;
	if (rows > 1 && cellH < targetWindowH + margin) {
		cellH = targetWindowH + margin;
	}
	CGFloat maxCombinedW = cellW - margin;
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
		CGFloat windowW = targetWindowW;
		CGFloat windowH = targetWindowH;
		if (windowW + sideChromeW > maxCombinedW) {
			windowW = maxCombinedW - sideChromeW;
			if (windowW < 1.0) {
				windowW = maxCombinedW;
			}
		}
		CGFloat maxCellH = cellH - margin;
		if (windowH > maxCellH) {
			windowH = maxCellH;
		}
		CGFloat cellX = rowNextX[row];
		CGFloat cellY = workArea.origin.y + row * cellH;
		CGPoint position = CGPointMake(cellX, cellY);
		CGSize size = CGSizeMake(windowW, windowH);
		AXValueRef positionValue = AXValueCreate(kAXValueCGPointType, &position);
		AXValueRef sizeValue = AXValueCreate(kAXValueCGSizeType, &size);
		activate_ax_window(windows[i]);
		if (positionValue != NULL) {
			AXUIElementSetAttributeValue(windows[i], kAXPositionAttribute, positionValue);
			CFRelease(positionValue);
		}
		if (sizeValue != NULL) {
			AXUIElementSetAttributeValue(windows[i], kAXSizeAttribute, sizeValue);
			CFRelease(sizeValue);
		}
		AXUIElementRef toolbar = copy_toolbar_for_main_window(windows[i]);
		if (toolbar != NULL) {
			toolbarWidths[i] = sideChromeW;
			CGPoint toolbarPosition = CGPointMake(
				cellX + visibleWindowW,
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
		AXValueRef finalPositionValue = AXValueCreate(kAXValueCGPointType, &finalPosition);
		if (finalPositionValue != NULL) {
			AXUIElementSetAttributeValue(windows[i], kAXPositionAttribute, finalPositionValue);
			CFRelease(finalPositionValue);
		}
		CGRect finalFrame = CGRectZero;
		if (copy_ax_window_frame(windows[i], &finalFrame) && finalFrame.size.width > 1.0) {
			finalX = finalFrame.origin.x;
			finalY = finalFrame.origin.y;
			finalMainW = finalFrame.size.width;
		}
		if (toolbars[i] != NULL) {
			CGPoint finalToolbarPosition = CGPointMake(
				finalX + finalMainW,
				finalY + 28.0
			);
			AXValueRef finalToolbarPositionValue = AXValueCreate(kAXValueCGPointType, &finalToolbarPosition);
			if (finalToolbarPositionValue != NULL) {
				AXUIElementSetAttributeValue(toolbars[i], kAXPositionAttribute, finalToolbarPositionValue);
				CFRelease(finalToolbarPositionValue);
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
	return adm_strdup("ok");
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
