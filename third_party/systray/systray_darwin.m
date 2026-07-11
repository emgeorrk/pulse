//go:build !ios

#import <Cocoa/Cocoa.h>
#include "systray.h"

#if __MAC_OS_X_VERSION_MIN_REQUIRED < 101400

    #ifndef NSControlStateValueOff
      #define NSControlStateValueOff NSOffState
    #endif

    #ifndef NSControlStateValueOn
      #define NSControlStateValueOn NSOnState
    #endif

#endif

@interface MenuItem : NSObject
{
  @public
    NSNumber* menuId;
    NSNumber* parentMenuId;
    NSString* title;
    NSString* tooltip;
    short disabled;
    short checked;
}
-(id) initWithId: (int)theMenuId
withParentMenuId: (int)theParentMenuId
       withTitle: (const char*)theTitle
     withTooltip: (const char*)theTooltip
    withDisabled: (short)theDisabled
     withChecked: (short)theChecked;
     @end
     @implementation MenuItem
     -(id) initWithId: (int)theMenuId
     withParentMenuId: (int)theParentMenuId
            withTitle: (const char*)theTitle
          withTooltip: (const char*)theTooltip
         withDisabled: (short)theDisabled
          withChecked: (short)theChecked
{
  menuId = [NSNumber numberWithInt:theMenuId];
  parentMenuId = [NSNumber numberWithInt:theParentMenuId];
  title = [[NSString alloc] initWithCString:theTitle
                                   encoding:NSUTF8StringEncoding];
  tooltip = [[NSString alloc] initWithCString:theTooltip
                                     encoding:NSUTF8StringEncoding];
  disabled = theDisabled;
  checked = theChecked;
  return self;
}
@end

@interface RightClickDetector : NSView

@property (copy) void (^onRightClicked)(NSEvent *);

@end

@implementation RightClickDetector

- (void)rightMouseUp:(NSEvent *)theEvent {
  if (!self.onRightClicked) {
    return;
  }

  self.onRightClicked(theEvent);
}

@end


// PATCH(pulse): view-backed menu row that keeps the menu open on click.
// AppKit dismisses the whole menu when a plain NSMenuItem fires its action;
// an item with a custom view owns mouse handling, and the menu only closes
// if the view asks for it — this view never does. Title, checkmark and
// hover highlight are drawn by the view, so [sync] must be called whenever
// the underlying NSMenuItem changes.
static const CGFloat kPulseItemHeight = 22;
static const CGFloat kPulseHighlightInsetX = 5;
static const CGFloat kPulseCheckX = 10;
static const CGFloat kPulseCheckSize = 14;
static const CGFloat kPulseTextX = 27;
static const CGFloat kPulseTrailingPad = 14;

@interface PulseKeepOpenItemView : NSView
- (instancetype)initWithMenuItem:(NSMenuItem *)theItem;
- (void)sync;
@end

@implementation PulseKeepOpenItemView
{
  __weak NSMenuItem *item;
  NSVisualEffectView *highlight;
  NSImageView *check;
  NSTextField *label;
}

- (instancetype)initWithMenuItem:(NSMenuItem *)theItem {
  self = [super initWithFrame:NSMakeRect(0, 0, 100, kPulseItemHeight)];
  if (!self) {
    return nil;
  }
  item = theItem;
  self.autoresizingMask = NSViewWidthSizable;

  highlight = [[NSVisualEffectView alloc] initWithFrame:NSInsetRect(self.bounds, kPulseHighlightInsetX, 0)];
  highlight.material = NSVisualEffectMaterialSelection;
  highlight.emphasized = YES;
  highlight.state = NSVisualEffectStateActive;
  highlight.blendingMode = NSVisualEffectBlendingModeBehindWindow;
  highlight.wantsLayer = YES;
  highlight.layer.cornerRadius = 4;
  highlight.layer.masksToBounds = YES;
  highlight.autoresizingMask = NSViewWidthSizable;
  highlight.hidden = YES;
  [self addSubview:highlight];

  NSImage *img = nil;
  if (@available(macOS 11.0, *)) {
    img = [NSImage imageWithSystemSymbolName:@"checkmark" accessibilityDescription:nil];
    img = [img imageWithSymbolConfiguration:
             [NSImageSymbolConfiguration configurationWithPointSize:11
                                                             weight:NSFontWeightBold]];
  }
  if (img == nil) {
    img = [NSImage imageNamed:NSImageNameMenuOnStateTemplate];
  }
  check = [NSImageView imageViewWithImage:img];
  check.frame = NSMakeRect(kPulseCheckX,
                           floor((kPulseItemHeight - kPulseCheckSize) / 2),
                           kPulseCheckSize, kPulseCheckSize);
  check.hidden = YES;
  [self addSubview:check];

  label = [NSTextField labelWithString:@""];
  label.font = [NSFont menuFontOfSize:13];
  label.lineBreakMode = NSLineBreakByClipping;
  label.maximumNumberOfLines = 1;
  [self addSubview:label];

  return self;
}

- (void)sync {
  NSMenuItem *it = item;
  if (!it) {
    return;
  }
  label.stringValue = it.title;
  [label sizeToFit];
  NSRect lf = label.frame;
  lf.origin.x = kPulseTextX;
  lf.origin.y = floor((kPulseItemHeight - NSHeight(lf)) / 2);
  label.frame = lf;

  check.hidden = (it.state != NSControlStateValueOn);
  self.alphaValue = it.enabled ? 1.0 : 0.4;

  // Grow-only so a shrinking live value doesn't make the open menu jitter.
  CGFloat needed = kPulseTextX + NSWidth(lf) + kPulseTrailingPad;
  if (needed > NSWidth(self.frame)) {
    NSRect f = self.frame;
    f.size.width = needed;
    self.frame = f;
  }
}

- (void)setHovered:(BOOL)hovered {
  highlight.hidden = !hovered;
  label.textColor = hovered ? [NSColor selectedMenuItemTextColor] : [NSColor labelColor];
  check.contentTintColor = hovered ? [NSColor selectedMenuItemTextColor] : [NSColor labelColor];
}

- (void)updateTrackingAreas {
  [super updateTrackingAreas];
  for (NSTrackingArea *area in self.trackingAreas) {
    [self removeTrackingArea:area];
  }
  NSTrackingArea *area = [[NSTrackingArea alloc]
      initWithRect:NSZeroRect
           options:(NSTrackingMouseEnteredAndExited | NSTrackingActiveAlways | NSTrackingInVisibleRect)
             owner:self
          userInfo:nil];
  [self addTrackingArea:area];
}

- (void)mouseEntered:(NSEvent *)event {
  if (((NSMenuItem *)item).enabled) {
    [self setHovered:YES];
  }
}

- (void)mouseExited:(NSEvent *)event {
  [self setHovered:NO];
}

- (void)viewDidMoveToWindow {
  [super viewDidMoveToWindow];
  if (self.window == nil) { // menu closed — mouseExited may never arrive
    [self setHovered:NO];
  }
}

- (void)mouseUp:(NSEvent *)event {
  NSMenuItem *it = item;
  if (!it || !it.enabled) {
    return;
  }
  // Deliberately no cancelTracking: the menu stays open, the Go side
  // toggles the checkbox through the usual Check/Uncheck path.
  systray_menu_item_selected([(NSNumber *)it.representedObject intValue]);
}

@end

@interface SystrayAppDelegate: NSObject <NSApplicationDelegate, NSMenuDelegate>
  - (void) add_or_update_menu_item:(MenuItem*) item;
  - (IBAction)menuHandler:(id)sender;
  - (void)menuWillOpen:(NSMenu*)menu;
  @property (assign) IBOutlet NSWindow *window;
@end

@implementation SystrayAppDelegate
{
  NSStatusItem *statusItem;
  NSMenu *menu;
  NSCondition* cond;
  // PATCH(pulse): template images for inline title icons, key → NSImage.
  NSMutableDictionary<NSString*, NSImage*> *titleIcons;
}

@synthesize window = _window;

- (void)applicationDidFinishLaunching:(NSNotification *)aNotification
{
  self->statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];

  self->menu = [[NSMenu alloc] init];
  self->menu.delegate = self;
  self->menu.autoenablesItems = FALSE;
  // Once the user has removed it, the item needs to be explicitly brought back,
  // even restarting the application is insufficient.
  // Since the interface from Go is relatively simple, for now we ensure it's
  // always visible at application startup.
  self->statusItem.visible = TRUE;

  NSStatusBarButton *button = self->statusItem.button;
  button.action = @selector(leftMouseClicked);

  [NSEvent addLocalMonitorForEventsMatchingMask: (NSEventTypeLeftMouseDown|NSEventTypeRightMouseDown)
                                        handler: ^NSEvent *(NSEvent *event) {
    if (event.window != self->statusItem.button.window) {
      return event;
    }

    if (event.modifierFlags & NSEventModifierFlagCommand) {
      return event;
    }

    [self leftMouseClicked];

    return nil;
  }];

  NSSize size = [button frame].size;
  NSRect frame = CGRectMake(0, 0, size.width, size.height);
  RightClickDetector *rightClicker = [[RightClickDetector alloc] initWithFrame:frame];
  rightClicker.onRightClicked = ^(NSEvent *event) {
    [self rightMouseClicked];
  };

  rightClicker.autoresizingMask = (NSViewWidthSizable |
                                   NSViewHeightSizable);
  button.autoresizesSubviews = YES;
  [button addSubview:rightClicker];

  systray_ready();
}

- (void)rightMouseClicked {
  systray_right_click();
}

- (void)leftMouseClicked {
  systray_left_click();
}

- (void)applicationWillTerminate:(NSNotification *)aNotification
{
  systray_on_exit();
}

- (void)setRemovalAllowed {
  NSStatusItemBehavior behavior = [self->statusItem behavior];
  behavior |= NSStatusItemBehaviorRemovalAllowed;
  self->statusItem.behavior = behavior;
}

- (void)setRemovalForbidden {
  NSStatusItemBehavior behavior = [self->statusItem behavior];
  behavior &= ~NSStatusItemBehaviorRemovalAllowed;
  // Ensure the menu item is visible if it was removed, since we're now
  // disallowing removal.
  self->statusItem.visible = TRUE;
  self->statusItem.behavior = behavior;
}

- (void)setIcon:(NSImage *)image {
  statusItem.button.image = image;
  [self updateTitleButtonStyle];
}

- (void)setTitle:(NSString *)title {
  statusItem.button.title = title;
  [self updateTitleButtonStyle];
}

// PATCH(pulse): attributed titles with inline icons. The status item font
// stays the source of truth; icons are drawn as text attachments sized to
// the font and vertically centered on the cap-height band, so they sit on
// the same baseline as the values next to them.
- (void)registerTitleIcon:(NSArray*)keyAndImage {
  if (titleIcons == nil) {
    titleIcons = [NSMutableDictionary dictionary];
  }
  titleIcons[[keyAndImage objectAtIndex:0]] = [keyAndImage objectAtIndex:1];
}

// Well above cap height: the glyphs are full-square symbolic icons and read
// too small when clamped to the caps band; 1.8 × cap height ≈ 17 pt with the
// default menu bar font, in line with typical status-item icons.
static const CGFloat kPulseTitleIconScale = 1.8;

// tintedTitleIcon fills the glyph's alpha with labelColor at draw time, so
// the icon follows the menu bar's effective appearance (light/dark wallpaper,
// theme switches) the same way the title text does. The text system ignores
// NSImage.template on attachments, so template rendering can't do this here.
// NSImage caches one representation per appearance, so a theme change just
// re-runs the handler.
static NSImage *tintedTitleIcon(NSImage *icon, CGFloat side) {
  return [NSImage imageWithSize:NSMakeSize(side, side)
                        flipped:NO
                 drawingHandler:^BOOL(NSRect dst) {
    [icon drawInRect:dst
            fromRect:NSZeroRect
           operation:NSCompositingOperationSourceOver
            fraction:1.0];
    // labelColor is ~15% translucent; rasterized without the vibrancy that
    // neighboring template status icons get, that reads gray on a light
    // menu bar. Resolve it under the current appearance and force it solid.
    NSColor *tint = [[NSColor.labelColor
        colorUsingColorSpace:NSColorSpace.sRGBColorSpace]
        colorWithAlphaComponent:1.0];
    [tint set];
    NSRectFillUsingOperation(dst, NSCompositingOperationSourceIn);
    return YES;
  }];
}

- (void)setTitleParts:(NSString *)encoded {
  NSFont *font = statusItem.button.font;
  if (font == nil) {
    font = [NSFont menuBarFontOfSize:0];
  }
  CGFloat side = ceil(font.capHeight * kPulseTitleIconScale);
  CGFloat drop = floor((side - font.capHeight) / 2.0);
  NSDictionary *textAttrs = @{NSFontAttributeName : font};
  NSMutableAttributedString *out = [[NSMutableAttributedString alloc] init];
  for (NSString *part in [encoded componentsSeparatedByString:@"\x1e"]) {
    NSArray<NSString*> *fields = [part componentsSeparatedByString:@"\x1f"];
    NSImage *icon = fields.count > 0 && fields[0].length > 0 ? titleIcons[fields[0]] : nil;
    if (icon != nil) {
      NSTextAttachment *att = [[NSTextAttachment alloc] init];
      att.image = tintedTitleIcon(icon, side);
      att.bounds = CGRectMake(0, -drop, side, side);
      [out appendAttributedString:[NSAttributedString attributedStringWithAttachment:att]];
    }
    if (fields.count > 1 && fields[1].length > 0) {
      [out appendAttributedString:[[NSAttributedString alloc] initWithString:fields[1]
                                                                  attributes:textAttrs]];
    }
  }
  statusItem.button.attributedTitle = out;
  [self updateTitleButtonStyle];
}

- (void)updateTitleButtonStyle {
  if (statusItem.button.image != nil) {
    if ([statusItem.button.title length] == 0) {
      statusItem.button.imagePosition = NSImageOnly;
    } else {
      statusItem.button.imagePosition = NSImageLeft;
    }
  } else {
    statusItem.button.imagePosition = NSNoImage;
  }
}


- (void)setTooltip:(NSString *)tooltip {
  statusItem.button.toolTip = tooltip;
}

- (IBAction)menuHandler:(id)sender
{
  NSNumber* menuId = [sender representedObject];
  systray_menu_item_selected(menuId.intValue);
}

- (void)menuWillOpen:(NSMenu *)menu {
  systray_menu_will_open();
}

- (void)add_or_update_menu_item:(MenuItem *)item {
  NSMenu *theMenu = self->menu;
  NSMenuItem *parentItem;
  if ([item->parentMenuId integerValue] > 0) {
    parentItem = find_menu_item(menu, item->parentMenuId);
    if (parentItem.hasSubmenu) {
      theMenu = parentItem.submenu;
    } else {
      theMenu = [[NSMenu alloc] init];
      [theMenu setAutoenablesItems:NO];
      [parentItem setSubmenu:theMenu];
      // PATCH(pulse): on macOS 14+ clicking a submenu parent that has an
      // action performs it and dismisses the whole menu; drop the action so
      // a click only opens the submenu.
      [parentItem setAction:nil];
    }
  }

  NSMenuItem *menuItem = find_menu_item(theMenu, item->menuId);
  if (menuItem == NULL) {
    menuItem = [theMenu addItemWithTitle:item->title
                               action:@selector(menuHandler:)
                        keyEquivalent:@""];
    [menuItem setRepresentedObject:item->menuId];
  }
  [menuItem setTitle:item->title];
  [menuItem setTag:[item->menuId integerValue]];
  [menuItem setTarget:self];
  [menuItem setToolTip:item->tooltip];
  if (item->disabled == 1) {
    menuItem.enabled = FALSE;
  } else {
    menuItem.enabled = TRUE;
  }
  if (item->checked == 1) {
    menuItem.state = NSControlStateValueOn;
  } else {
    menuItem.state = NSControlStateValueOff;
  }
  // PATCH(pulse): keep submenu parents action-free on the update path too.
  if (menuItem.hasSubmenu) {
    [menuItem setAction:nil];
  }
  // PATCH(pulse): view-backed rows draw title/state themselves.
  if ([menuItem.view isKindOfClass:[PulseKeepOpenItemView class]]) {
    [(PulseKeepOpenItemView *)menuItem.view sync];
  }
}

// PATCH(pulse): swap a plain row for a view-backed one that keeps the menu
// open on click (see PulseKeepOpenItemView).
- (void)set_menu_item_keep_open:(NSNumber *)menuId
{
  NSMenuItem *menuItem = find_menu_item(menu, menuId);
  if (menuItem == NULL || menuItem.hasSubmenu || menuItem.view != nil) {
    return;
  }
  PulseKeepOpenItemView *view = [[PulseKeepOpenItemView alloc] initWithMenuItem:menuItem];
  menuItem.view = view;
  [view sync];
}

NSMenuItem *find_menu_item(NSMenu *ourMenu, NSNumber *menuId) {
  NSMenuItem *foundItem = [ourMenu itemWithTag:[menuId integerValue]];
  if (foundItem != NULL) {
    return foundItem;
  }
  NSArray *menu_items = ourMenu.itemArray;
  int i;
  for (i = 0; i < [menu_items count]; i++) {
    NSMenuItem *i_item = [menu_items objectAtIndex:i];
    if (i_item.hasSubmenu) {
      foundItem = find_menu_item(i_item.submenu, menuId);
      if (foundItem != NULL) {
        return foundItem;
      }
    }
  }

  return NULL;
};

- (void) add_separator:(NSNumber*) parentMenuId
{
  if (parentMenuId.integerValue != 0) {
    NSMenuItem* menuItem = find_menu_item(menu, parentMenuId);
    if (menuItem != NULL) {
      [menuItem.submenu addItem: [NSMenuItem separatorItem]];
      return;
    }
  }
  [menu addItem: [NSMenuItem separatorItem]];
}

- (void) hide_menu_item:(NSNumber*) menuId
{
  NSMenuItem* menuItem = find_menu_item(menu, menuId);
  if (menuItem != NULL) {
    [menuItem setHidden:TRUE];
  }
}

- (void) setMenuItemIcon:(NSArray*)imageAndMenuId {
  NSImage* image = [imageAndMenuId objectAtIndex:0];
  NSNumber* menuId = [imageAndMenuId objectAtIndex:1];

  NSMenuItem* menuItem;
  menuItem = find_menu_item(menu, menuId);
  if (menuItem == NULL) {
    return;
  }
  menuItem.image = image;
}

// PATCH(pulse): live style switching needs a way back to no icon;
// setMenuItemIcon can't express it (nil in the NSArray literal would throw).
- (void) clearMenuItemIcon:(NSNumber*)menuId {
  NSMenuItem* menuItem = find_menu_item(menu, menuId);
  if (menuItem != NULL) {
    menuItem.image = nil;
  }
}

- (void)show_menu
{
  // Attach the menu and synthesize a click so AppKit positions it natively,
  // then detach it in menuDidClose: so the next click reaches the button
  // action (and the Go tap handlers) again.
  self->statusItem.menu = self->menu;
  [self->statusItem.button performClick:nil];
}

- (void)menuDidClose:(NSMenu *)menu {
  // Defer the detach so we don't pull the menu out from under AppKit
  // while it is still tearing the menu down.
  dispatch_async(dispatch_get_main_queue(), ^{
    self->statusItem.menu = nil;
  });
}

- (void) show_menu_item:(NSNumber*) menuId
{
  NSMenuItem* menuItem = find_menu_item(menu, menuId);
  if (menuItem != NULL) {
    [menuItem setHidden:FALSE];
  }
}

- (void) remove_menu_item:(NSNumber*) menuId
{
  NSMenuItem* menuItem = find_menu_item(menu, menuId);
  if (menuItem != NULL) {
    [menuItem.menu removeItem:menuItem];
  }
}

- (void) reset_menu
{
  [self->menu removeAllItems];
}

- (void) quit
{
  // This tells the app event loop to stop after processing remaining messages.
  [NSApp stop:self];
  // The event loop won't return until it processes another event.
  // https://stackoverflow.com/a/48064752/149482
  NSPoint eventLocation = NSMakePoint(0, 0);
  NSEvent *customEvent = [NSEvent otherEventWithType:NSEventTypeApplicationDefined
                                            location:eventLocation
                                       modifierFlags:0
                                           timestamp:0
                                        windowNumber:0
                                             context:nil
                                             subtype:0
                                               data1:0
                                               data2:0];
  [NSApp postEvent:customEvent atStart:NO];
}

@end

bool internalLoop = false;
SystrayAppDelegate *owner;

void setInternalLoop(bool i) {
	internalLoop = i;
}

void registerSystray(void) {
  if (!internalLoop) { // with an external loop we don't take ownership of the app
    return;
  }

  owner = [[SystrayAppDelegate alloc] init];
  [[NSApplication sharedApplication] setDelegate:owner];

  // A workaround to avoid crashing on macOS versions before Catalina. Somehow
  // SIGSEGV would happen inside AppKit if [NSApp run] is called from a
  // different function, even if that function is called right after this.
  if (floor(NSAppKitVersionNumber) <= /*NSAppKitVersionNumber10_14*/ 1671){
    [NSApp run];
  }
}

void nativeEnd(void) {
  systray_on_exit();
}

int nativeLoop(void) {
  if (floor(NSAppKitVersionNumber) > /*NSAppKitVersionNumber10_14*/ 1671){
    [NSApp run];
  }
  return EXIT_SUCCESS;
}

void nativeStart(void) {
  owner = [[SystrayAppDelegate alloc] init];

  NSNotification *launched = [NSNotification notificationWithName:NSApplicationDidFinishLaunchingNotification
                                                        object:[NSApplication sharedApplication]];
  [owner applicationDidFinishLaunching:launched];
}

void runInMainThread(SEL method, id object) {
  [owner
    performSelectorOnMainThread:method
                     withObject:object
                  waitUntilDone: YES];
}

void setIcon(const char* iconBytes, int length, bool template) {
  NSData* buffer = [NSData dataWithBytes: iconBytes length:length];
  @autoreleasepool {
    NSImage *image = [[NSImage alloc] initWithData:buffer];
    [image setSize:NSMakeSize(16, 16)];
    image.template = template;
    runInMainThread(@selector(setIcon:), (id)image);
  }
}

void setMenuItemIcon(const char* iconBytes, int length, int menuId, bool template) {
  NSData* buffer = [NSData dataWithBytes: iconBytes length:length];
  @autoreleasepool {
    NSImage *image = [[NSImage alloc] initWithData:buffer];
    [image setSize:NSMakeSize(16, 16)];
    image.template = template;
    NSNumber *mId = [NSNumber numberWithInt:menuId];
    runInMainThread(@selector(setMenuItemIcon:), @[image, (id)mId]);
  }
}

void setTitle(char* ctitle) {
  NSString* title = [[NSString alloc] initWithCString:ctitle
                                             encoding:NSUTF8StringEncoding];
  free(ctitle);
  runInMainThread(@selector(setTitle:), (id)title);
}

// PATCH(pulse): inline title icons. Only the alpha channel is used — the
// glyph is tinted at draw time (see tintedTitleIcon), not template-rendered.
void registerTitleIcon(char* key, const char* iconBytes, int length) {
  NSString* nsKey = [[NSString alloc] initWithCString:key
                                             encoding:NSUTF8StringEncoding];
  free(key);
  NSData* buffer = [NSData dataWithBytes:iconBytes length:length];
  @autoreleasepool {
    NSImage *image = [[NSImage alloc] initWithData:buffer];
    if (image == nil) {
      return;
    }
    runInMainThread(@selector(registerTitleIcon:), @[nsKey, image]);
  }
}

void setTitleParts(char* cencoded) {
  NSString* encoded = [[NSString alloc] initWithCString:cencoded
                                               encoding:NSUTF8StringEncoding];
  free(cencoded);
  runInMainThread(@selector(setTitleParts:), (id)encoded);
}

void clearMenuItemIcon(int menuId) {
  NSNumber *mId = [NSNumber numberWithInt:menuId];
  runInMainThread(@selector(clearMenuItemIcon:), (id)mId);
}

void setTooltip(char* ctooltip) {
  NSString* tooltip = [[NSString alloc] initWithCString:ctooltip
                                               encoding:NSUTF8StringEncoding];
  free(ctooltip);
  runInMainThread(@selector(setTooltip:), (id)tooltip);
}

void setRemovalAllowed(bool allowed) {
  if (allowed) {
    runInMainThread(@selector(setRemovalAllowed), nil);
  } else {
    runInMainThread(@selector(setRemovalForbidden), nil);
  }
}

void add_or_update_menu_item(int menuId, int parentMenuId, char* title, char* tooltip, short disabled, short checked, short isCheckable) {
  MenuItem* item = [[MenuItem alloc] initWithId: menuId withParentMenuId: parentMenuId withTitle: title withTooltip: tooltip withDisabled: disabled withChecked: checked];
  free(title);
  free(tooltip);
  runInMainThread(@selector(add_or_update_menu_item:), (id)item);
}

void add_separator(int menuId, int parentId) {
  NSNumber *pId = [NSNumber numberWithInt:parentId];
  runInMainThread(@selector(add_separator:), (id)pId);
}

// PATCH(pulse): see PulseKeepOpenItemView.
void set_menu_item_keep_open(int menuId) {
  NSNumber *mId = [NSNumber numberWithInt:menuId];
  runInMainThread(@selector(set_menu_item_keep_open:), (id)mId);
}

void hide_menu_item(int menuId) {
  NSNumber *mId = [NSNumber numberWithInt:menuId];
  runInMainThread(@selector(hide_menu_item:), (id)mId);
}

void remove_menu_item(int menuId) {
  NSNumber *mId = [NSNumber numberWithInt:menuId];
  runInMainThread(@selector(remove_menu_item:), (id)mId);
}

void show_menu() {
  runInMainThread(@selector(show_menu), nil);
}

void show_menu_item(int menuId) {
  NSNumber *mId = [NSNumber numberWithInt:menuId];
  runInMainThread(@selector(show_menu_item:), (id)mId);
}

void reset_menu() {
  runInMainThread(@selector(reset_menu), nil);
}

void quit() {
  runInMainThread(@selector(quit), nil);
}
