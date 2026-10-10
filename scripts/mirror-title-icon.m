#import <AppKit/AppKit.h>

// Loaded only into the mirror child process. The document-proxy position is
// the native macOS location for an image immediately before a window title.
static void applyMirrorTitleIcon(NSWindow *window, NSImage *image, NSURL *url,
                                 NSString *title) {
    if (!window || ![window.title isEqualToString:title]) return;
    if ([window.representedURL isEqual:url]) return;
    window.representedURL = url;
    NSButton *button = [window standardWindowButton:NSWindowDocumentIconButton];
    if (!button) {
        window.representedURL = nil;
        return;
    }
    button.image = image;
    button.enabled = NO; // No file navigation or drag action on this type icon.
    fprintf(stderr, "INFO: Device type title icon applied\n");
}

__attribute__((constructor)) static void configureMirrorTitleIcon(void) {
    const char *path = getenv("AVDM_MIRROR_ICON");
    const char *text = getenv("AVDM_MIRROR_TITLE");
    if (!path || !text) return;
    @autoreleasepool {
        NSString *iconPath = [NSString stringWithUTF8String:path];
        NSString *title = [NSString stringWithUTF8String:text];
        NSImage *image = [[NSImage alloc] initWithContentsOfFile:iconPath];
        if (!image) return;
        image.size = NSMakeSize(16, 16);
        image.template = NO;
        NSURL *url = [NSURL fileURLWithPath:iconPath];
        dispatch_async(dispatch_get_main_queue(), ^{
            NSNotificationCenter *center = NSNotificationCenter.defaultCenter;
            for (NSString *name in @[NSWindowDidBecomeKeyNotification,
                                      NSWindowDidBecomeMainNotification,
                                      NSWindowDidUpdateNotification]) {
                [center addObserverForName:name object:nil queue:nil
                                usingBlock:^(NSNotification *notification) {
                    if ([notification.object isKindOfClass:NSWindow.class]) {
                        applyMirrorTitleIcon(notification.object, image, url, title);
                    }
                }];
            }
            for (NSWindow *window in NSApp.windows) {
                applyMirrorTitleIcon(window, image, url, title);
            }
        });
    }
}
