/*
**	InvenWin_cl.cp		ClanLord
**
**	Copyright 2023 Delta Tao Software, Inc.
** 
**	Licensed under the Apache License, Version 2.0 (the "License");
**	you may not use this file except in compliance with the License.
**	You may obtain a copy of the License at
** 
**     https://www.apache.org/licenses/LICENSE-2.0
** 
**	Unless required by applicable law or agreed to in writing, software
**	distributed under the License is distributed on an "AS IS" BASIS,
**	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
**	See the License for the specific language governing permissions and
**	imitations under the License.
*/

#include "ListView_cl.h"
#include "SendText_cl.h"
#include "TextCmdList_cl.h"

/*
**	Entry Routines
*/
/*
void		CreateInvenWindow( const DTSRect * pos );
void		ResetInventory();
void		AddToInventory( DTSKeyID id, int idx, bool equipped );
size_t		GetInvItemName( int id, char * dst, size_t maxlen );
void		ClickSlotObject( int slot );
void		ClearInvCmd();
bool		InventoryShortcut( int ch, uint modifiers );
DTSError	EquipNamedItem( const char * name );
DTSError	SendInvCmd( long inCmd, int idx );
void		UpdateInventoryWindow();
DTSError	UpdateItemPrefs();
void		InventorySetCustomName( DTSKeyID id, int idx, const char * text );
*/


/*
**	Definitions
*/

// are we using a CFDictionary to store item short-cut keys?
#define ITEM_PREFS_IN_PLIST		0
#define pref_InventoryShortcut	CFSTR("InventoryItemQuickKeys")

// originally this code used kTypeItem, but the definition of that symbol
// has changed, thus rendering saved player prefs invalid.
// So I've changed all references to kTypeItem herein to kTypeItemPrefs,
// which is defined to have the same value as is found in all our players'
// preferences. That insulates us from internal server/editor changes that
// we shouldn't have to care about.

const DTSKeyType kTypeItemPrefs = 'Itm7';


// if you want to test equipping items by name, via AppleEvents, set this to 1.
// also make the corresponding change in ClanLord.sdef, something like...
//		<command name="equip item" code="CLc∆Eqip"
//			description="Transfer an item from your backpack into your hand.">
//		  <direct-parameter type="text" description="The full name of the item to equip"/>
//		</command>

#define TEST_EQUIP_ITEM				0


// this controls which variety of text-comparison APIs we use for inventory-name
// sorting & testing: "modern" CFStrings, or MacOS Classic-era routines, or
// standard C library functions like strcmp() & friends. The latter is apt to produce
// poor results with names containing extended MacRoman characters. However, they
// also minimize porting hurdles...
// FIXME: extend this to the player window, macros, etc.
#if defined( DTS_Mac )
# define USE_CFSTRING_COMPARE		1		// use CFStrings
# define USE_MACOS_STRING_COMPARE	0		// use obsolete <StringCompare.h>
	// if both are off, we use <string.h> et al.
#endif


const int kMaxItems				= 32;	// this needs to stay in sync with the real definition
										// (in Public_cle.h)

const int kMaxItemCustomNameLen = 51;	// ditto, in scripts/constants/misc.h
										// 50 characters displayed plus one for the null
										
const uint kItemFlagData		= 0x0400;	// is this a template-date item?
										// (must match kItemTypeHasData in editor/server sources)

// window-geometry constants
const int kListWidth			= 200;	// this now matches the text pane & players window
const int kItemListHeight		= 18;	// same as players list

// resize limits
const int kMinInvContHeight		= 4 * kItemListHeight;
const int kMaxInvContHeight		= kMaxItems * kItemListHeight;

const int kSBarWidth			= 15;
const int kMaxInvWinWidth		= kListWidth + kSBarWidth;
const int kMinInvWinWidth		= kMaxInvWinWidth;	// previously was (128 + kSBarWidth)
const int kMinInvWinHeight		= kMinInvContHeight;
const int kMaxInvWinHeight		= kMaxInvContHeight;


#if USE_CFSTRING_COMPARE
// case-insensitivity is always available, but diacritic-insensitive comparison
// isn't available before the 10.5 SDK
const CFStringCompareFlags kItemNameCompareFlags =
# if MAC_OS_X_VERSION_MAX_ALLOWED >= 1050
								kCFCompareDiacriticInsensitive |
# endif
								kCFCompareCaseInsensitive;
#endif  // USE_CFSTRING_COMPARE


//
// To be stored in the prefs, as kTypeItemPrefs
//
struct SItemPrefs
{
	char	itemCmdKey;
	char	filler;		// pad out to 2 bytes total, to match the traditional size of this
						// struct, back when it was wrapped in a #pragma align=mac68k block
};


/*
**	ItemList class -- info about a single inventory item
*/
class ItemList : public DTSLinkedList<ItemList>
{
public:
	const DTSKeyID	itemID;				// its (invariant) KeyID
	const ClientItem	itemData;		// flags / images / public name / etc.
#if USE_CFSTRING_COMPARE
	CFStringRef		itemCFName;			// public name, cached as a CFString
#endif
	char			itemCustomName[ kMaxItemCustomNameLen ]; // may be non-empty for tagged items
	int				itemIndex;								// may be non-zero  for tagged items
	int				itemQuantity;
	char			itemCmdKey;
	bool			itemDirty;			// need to resave user's shortcut key?
	bool			itemEquipped;
	
				// constructor / destructor
				ItemList( DTSKeyID itmID, const ClientItem& data, bool equip );
	
				~ItemList()
						{
#if USE_CFSTRING_COMPARE
						if ( itemCFName )
							CFRelease( itemCFName );
#endif  // USE_CFSTRING_COMPARE
						}
	
	void	GetCanonicalName( char * oBuff, size_t bufLen, bool caps = false ) const;
	static void	CapitalizeItemName( char * name );
};

// syntactic convenience
typedef ItemList * ItemListPtr;


/*
**	view describing the scrollable list of inventory items
*/
class InvenList : public ListView
{
	ItemList *		icSelect;		// currently-selected item, or NULL
	
public:
						InvenList() :
							icSelect( nullptr )
							{}
						
	// overloaded routines
	virtual void		DrawNode( int inIndex, const DTSRect& inRect, bool bActive );
	virtual bool		ClickNode( int inIdx, const DTSPoint& pt, ulong t, uint mods );
	virtual bool		KeyStroke( int ch, uint modifiers );
	
	ItemList *			Selected() const { return icSelect; }
	void				SetSelection( ItemList * item ) { icSelect = item; }
};


/*
**	view describing the entire interior of the inventory window
*/
class InvenContent : public DTSView
{
	const int		icTopMargin;			// space used by top placard(s) if any
	const int		icBottomMargin;			// ditto, by bottom
	bool			icActive;				// is window active?
	
public:
						InvenContent();
	
	// overloaded routines
	virtual void		DoDraw();
	virtual void		Size( DTSCoord h, DTSCoord v );
//	virtual bool		Click( const DTSPoint& start, ulong time, uint modifiers );

	void				Activate()		{ icActive = true; }
	void				Deactivate()	{ icActive = false; }
	DTSPoint			BestSize( DTSPoint s ) const;
	int					Margins() const { return icTopMargin + icBottomMargin; }
};


#pragma mark class InvenWindow
/*
**	class InvenWindow
*/
class InvenWindow : public DTSWindow, public CarbonEventResponder
{
public:
	InvenContent	iwContent;
	
	// overloaded routines
	virtual DTSError	Init( uint kind );
	virtual void		Activate();
	virtual void		Deactivate();
	virtual void		Size( DTSCoord h, DTSCoord v );
	
	virtual bool		KeyStroke( int ch, uint modifiers );
	virtual bool		DoCmd( long cmd );
	
	void				UpdateInvenMenu() const;
	
	OSStatus			DoWindowEvent( const CarbonEvent& );
	virtual OSStatus	HandleEvent( CarbonEvent& );
};


/*
**	Internal Routines
*/
static bool			SortInventoryList();
static void			InventorySave( bool inAll );
static int	 		CountInventoryItems( bool wantDups = false );
static ItemList *	FindItemByIDAndIndex( DTSKeyID id, int idx = 0 );
static void			RenumberItemIndex( DTSKeyID itemID, int idx, bool upward );
static void			BumpItemsFromSlot( DTSKeyID id, int slot, int idx = 0 );

#if TEST_EQUIP_ITEM
static DTSError		InstallEquipItemHandler();
#endif


/*
**	Internal Variables
*/
static ItemList *		gRootInvItem;
static InvenList		gInvenList;

#if ITEM_PREFS_IN_PLIST
static	CFMutableDictionaryRef	sItemShortcutKeys;
#endif

// temp. pending mannequin or whatever
DTSKeyID	gSelectedItemPictID = 0;


/*
**	CreateInvenWindow()
**
**	Create and show the inventory window.
*/
void
CreateInvenWindow( const DTSRect * pos )
{
#if ITEM_PREFS_IN_PLIST
	if ( not sItemShortcutKeys )
		{
		sItemShortcutKeys = CFDictionaryCreateMutable( kCFAllocatorDefault,
								 0,
								 &kCFTypeDictionaryKeyCallBacks,
								 &kCFTypeDictionaryValueCallBacks );
		__Check( sItemShortcutKeys );
		}
#endif  // ITEM_PREFS_IN_PLIST
	
	// create the window
	InvenWindow * win = NEW_TAG("InvenWindow") InvenWindow;
	CheckPointer( win );
	gInvenWindow = win;
	
	// create window of type document with close, zoom, and grow boxes.
	DTSError result = win->Init( kWKindDocument
								 | kWKindCloseBox | kWKindZoomBox | kWKindGrowBox
								 | kWKindGetFrontClick /* | kWKindCompositing */
								 | kWKindLiveResize );
	if ( result )
		return;
	
	// init the list
	gInvenList.Init( &win->iwContent, win, kItemListHeight );
	
	// set the title and position it
	win->SetTitle( _(TXTCL_INVENTORY_WIN_TITLE) );		// "Inventory"
	win->SetZoomSize( kMaxInvWinWidth, kMaxInvWinHeight );
	int top  = pos->rectTop;
	int left = pos->rectLeft;
	
	win->Move( left, top );
	DTSPoint prefSz = DTSPoint( pos->rectRight - left, pos->rectBottom - top );
	prefSz = win->iwContent.BestSize( prefSz );
	win->Size( prefSz.ptH, prefSz.ptV );
	
	win->iwContent.Show();
	gInvenList.Show();
	win->Show();
	
#if TEST_EQUIP_ITEM
	// testing
	InstallEquipItemHandler();
#endif
	
	ResetInventory();
}


/*
**	InvenWin::Init()
*/
DTSError
InvenWindow::Init( uint kind )
{
	OSStatus result = DTSWindow::Init( kind );
	if ( noErr != result )
		return result;
	
	iwContent.Attach( this );
	
	// set resize limits
	HISize minSz = CGSizeMake( kMinInvWinWidth, kMinInvWinHeight );
	HISize maxSz = CGSizeMake( kMaxInvWinWidth, kMaxInvWinHeight );
	__Verify_noErr( SetWindowResizeLimits( DTS2Mac(this), &minSz, &maxSz ) );
	
	// prepare Carbon events
	result = InitTarget( GetWindowEventTarget( DTS2Mac(this) ) );
	if ( noErr == result )
		{
		const EventTypeSpec evts[] =
			{
				{ kEventClassWindow, kEventWindowDrawContent },
				{ kEventClassWindow, kEventWindowBoundsChanged },
				{ kEventClassWindow, kEventWindowBoundsChanging }
			};
		
		result = AddEventTypes( GetEventTypeCount( evts ), evts  );
		}
	
	return result;
}


/*
**	InvenWindow::Activate()
**
**	check the menu
**	activate the window
*/
void
InvenWindow::Activate()
{
	gDTSMenu->SetFlags( mInvenWindow, kMenuChecked );
	
	DTSWindow::Activate();
	
	iwContent.Activate();
	Invalidate();
}


/*
**	InvenWindow::Deactivate()
**
**	uncheck the menu
**	deactivate the window
*/
void
InvenWindow::Deactivate()
{
	gDTSMenu->SetFlags( mInvenWindow, kMenuNormal );
	
	iwContent.Deactivate();
	Invalidate();
	
	DTSWindow::Deactivate();
}


/*
**	InvenWindow::Size()
**
**	move the future scroll bars
*/
void
InvenWindow::Size( DTSCoord width, DTSCoord height )
{
	// avoid useless work
	DTSRect bounds;
	GetBounds( &bounds );
	if ( width  == bounds.rectRight - bounds.rectLeft
	&&   height == bounds.rectBottom - bounds.rectTop )
		{
		return;
		}
	
	// call through
	DTSWindow::Size( width, height );
	
	SInt32 sbarWd( 15 ), sbarInset( 0 ), growboxHt( 15 );
	::GetThemeMetric( kThemeMetricScrollBarWidth, &sbarWd );
	::GetThemeMetric( kThemeMetricScrollBarOverlap, &sbarInset );
	::GetThemeMetric( kThemeMetricResizeControlHeight, &growboxHt );
	
	// size the content
	iwContent.Size( width - sbarWd + sbarInset, height );
	
	// place & size the scrollbar
	gInvenList.lvScrollBar.Move( width - sbarWd + sbarInset, -sbarInset );
	gInvenList.lvScrollBar.Size( sbarWd, height - growboxHt + 2 * sbarInset);
}


/*
**	InvenWindow::KeyStroke()
**
**	pass keys to the list and then game window
*/
bool
InvenWindow::KeyStroke( int ch, uint modifiers )
{
	// if it's a cmd-key, let app handle it
	if ( modifiers & kKeyModMenu )
		return false;
	
	// if the list can handle it, we're done
	if ( gInvenList.KeyStroke( ch, modifiers ) )
		return true;
	
	// otherwise, summon up the game window and let it have a crack
	if ( DTSWindow * win = gGameWindow )
		{
		win->GoToFront();
		return win->KeyStroke( ch, modifiers );
		}
	
	return false;
}


/*
**	InvenWindow::DoCmd()
**
**	handle a menu selection
*/
bool
InvenWindow::DoCmd( long cmd )
{
	const ItemList * item = gInvenList.Selected();
	bool result = true;
	char buff[ 256 ];
	
	switch ( cmd )
		{
//		case cmd_Equip:
		case mEquipItem:
			if ( item )
				{
				DTSKeyID itemid = item->itemID;
				bool equipped = item->itemEquipped;
				if ( equipped )
					snprintf( buff, sizeof buff, "\\UNEQUIP %d", (int) itemid );
				else
					snprintf( buff, sizeof buff, "\\EQUIP %d %d",
						(int) itemid, (int )item->itemIndex + 1 );
				SendText( buff, true );
				}
			break;
		
//		case cmd_Use:
		case mUseItem:
			if ( item )
				{
				DTSKeyID itemid = item->itemID;
				snprintf( buff, sizeof buff, "\\USEITEM %d", (int) itemid );
				SendText( buff, true );
				}
			break;
		
		default:
			result = DTSWindow::DoCmd( cmd );
		}
	
	return result;
}


/*
**	InvenWindow::UpdateInvenMenu()
**
**	keep the inventory menu items in sync
*/
void
InvenWindow::UpdateInvenMenu() const
{
	DTSMenu * menu = gDTSMenu;
	
	if ( const ItemList * item = gInvenList.Selected() )
		{
#ifdef MULTILINGUAL
		// extract translation of item (if any)
		char itemname[ kMaxItemNameLen ];
		const char * itempnt = item->itemData.itemName;
		int itemnameidx = ExtractLangText( &itempnt, gRealLangID, nullptr );
		StringCopySafe( itemname, itempnt, itemnameidx + 1 );
		itemname[ itemnameidx ] = '\0';
		
		if ( '\0' == *itemname )
			{
			// "<unnamed item>"
			StringCopySafe( itemname, _(TXTCL_UNNAMED_ITEM), sizeof itemname );
			}
#else
		const char * itemname = item->itemData.itemName;
		if ( '\0' == *itemname )
			{
			// "<unnamed item>"
			itemname = _(TXTCL_UNNAMED_ITEM);
			}
#endif // MULTILINGUAL
		
		char buff[ 256 ];
		snprintf( buff, sizeof buff, "%s %s",
			item->itemEquipped ? _(TXTCL_UNEQUIP) : _(TXTCL_EQUIP),		// "Unequip" : "Equip"
			itemname );
		menu->SetText( mEquipItem, buff );
		snprintf( buff, sizeof buff, _(TXTCL_USE_ITEM), itemname );		// "Use %s"
		menu->SetText( mUseItem, buff );
		
		bool canEquip = (item->itemData.itemSlot >= kItemSlotFirstReal &&
						 item->itemData.itemSlot <= kItemSlotLastReal);
		menu->SetFlags( mEquipItem, canEquip ? kMenuNormal : kMenuDisabled );
		menu->SetFlags( mUseItem, kMenuNormal );
		
		DTSKeyID pictID = item->itemData.itemWornPictID;
		if ( not pictID )
			pictID = 1175;	// alpha '?' image
		gSelectedItemPictID = pictID;
		}
	else
		{
		menu->SetText( mEquipItem, _(TXTCL_EQUIP) );	// "Equip"
		menu->SetText( mUseItem, _(TXTCL_USE) );		// "Use"
		menu->SetFlags( mEquipItem, kMenuDisabled );
		menu->SetFlags( mUseItem, kMenuDisabled );
		
		gSelectedItemPictID = 0;
		}
}


/*
**	InvenWindow::DoWindowEvent()
*/
OSStatus
InvenWindow::DoWindowEvent( const CarbonEvent& evt )
{
	OSStatus result( eventNotHandledErr );
	OSStatus err( noErr );
	UInt32 attr( 0 );
	HIRect r;
	
	switch ( evt.Kind() )
		{
		case kEventWindowDrawContent:
			DoDraw();
			result = noErr;
			break;
		
		case kEventWindowBoundsChanged:
			{
			evt.GetParameter( kEventParamAttributes, typeUInt32, sizeof attr, &attr );
			
			if ( attr & kWindowBoundsChangeSizeChanged )
				{
				err = evt.GetParameter( kEventParamCurrentBounds,
								typeHIRect,sizeof r, &r );
				__Check_noErr( err );
				if ( noErr == err )
					{
					StDrawEnv saveEnv( this );
					Size( CGRectGetWidth( r ), CGRectGetHeight( r ) );
					DoDraw();
					result = noErr;
					}
				else
					result = err;
				}
			}
			break;
		
		case kEventWindowBoundsChanging:
			{
			const UInt32 kOnlyThese = kWindowBoundsChangeSizeChanged
									| kWindowBoundsChangeUserResize;
			
			err = evt.GetParameter( kEventParamCurrentBounds, typeHIRect, sizeof r, &r );
			if ( noErr == err )
				err = evt.GetParameter( kEventParamAttributes, typeUInt32, sizeof attr, &attr );
			if ( noErr == err
			&&   (attr & kOnlyThese) == kOnlyThese )
				{
				DTSPoint sz = DTSPoint( CGRectGetWidth( r ), CGRectGetHeight( r ) );
				sz = iwContent.BestSize( sz );
				r.size = CGSizeMake( sz.ptH, sz.ptV );
				evt.SetParameter( kEventParamCurrentBounds, typeHIRect, sizeof r, &r );
				result = noErr;
				}
			}
			break;
		}
	
	return result;
}


/*
**	InvenWindow::HandleEvent()
*/
OSStatus
InvenWindow::HandleEvent( CarbonEvent& evt )
{
	OSStatus result( eventNotHandledErr );
	
	switch ( evt.Class() )
		{
		case kEventClassWindow:
			result = DoWindowEvent( evt );
			break;
		}
	
	if ( eventNotHandledErr == result )
		result = CarbonEventResponder::HandleEvent( evt );
	
	return result;
}

#pragma mark -


/*
**	InvenContent::InvenContent()
**
**	constructor
*/
InvenContent::InvenContent() :
		icTopMargin( 0 ),			// nothing on top for now
		icBottomMargin( 16 ),		// height for one placard
		icActive( false )
		{}


/*
**	InvenContent::Size()
**
**	size the content
*/
void
InvenContent::Size( DTSCoord width, DTSCoord height )
{
	// resize items list
	gInvenList.Size( width, height - Margins() );
	gInvenList.Move( 0, icTopMargin );
	
	// Change the height to that actually used
	DTSRect bounds;
	gInvenList.GetBounds( &bounds );
	height = Margins() + bounds.rectBottom - bounds.rectTop;
	
	// inherited
	DTSView::Size( width, height );
}



/*
**	InvenContent::DoDraw()
**	draw the inventory content (basically, all -but- the scrolling list thingy)
*/
void
InvenContent::DoDraw()
{
	const ThemeFontID kFont = kThemeLabelFont;
	
//	Reset();
	
	DTSRect bounds;
	GetBounds( &bounds );
	Erase( &bounds );
	ThemeDrawState state = icActive ? kThemeStateActive : kThemeStateInactive;
	
	// draw the top placard
/*	there isn't one
	if ( icTopMargin )
		{
		}
*/
	
	// draw the bottom placard
	if ( icBottomMargin )
		{
		// fix our canvas
		DTSRect box = bounds;
		box.rectTop = box.rectBottom - icBottomMargin;
		box.Offset( 0, 1 );
		
		{
		Rect b = *DTS2Mac(&box);
		// tuck the placard's edges out of sight
		--b.left;
		++b.right;
		(void) DrawThemePlacard( &b, state );
		}
		
		// determine capacities
		int nSlots = CountInventoryItems( true );		// total # of slots in use
//		int nItems = CountInventoryItems( false );		// # of distinct items
		int freeSlots = kMaxItems - nSlots;
		
		// "%d items. "
		CFStringRef buf1 = CFStringCreateFormatted( CFSTR( _(TXTCL_NR_ITEMS) ), nSlots );
		int wid1 = GetTextBoxWidth( this, buf1, kFont );
		
		CFStringRef buf2( nullptr );
		if ( freeSlots )
			{
			buf2 = CFStringCreateFormatted( 
				// "%d slot free" : "%d slots free"
				1 == freeSlots ? CFSTR( _(TXTCL_ONE_SLOT_FREE)) : CFSTR(_(TXTCL_X_SLOTS_FREE)),
				freeSlots );
			}
		else
			{
			// "Your pack is full."
			buf2 = CFSTR( _(TXTCL_PACK_FULL) );
			CFRetain( buf2 );  // to balance the release, below
			}
		
		int wid2 = GetTextBoxWidth( this, buf2, kFont );
		int left = ( (box.rectRight - box.rectLeft) - (wid1 + wid2) ) / 2;
		
		DTSColor fgColor = DTSColor::black;
		GetThemeTextColor( icActive ? kThemeTextColorPlacardActive
									: kThemeTextColorPlacardInactive,
							32, true, DTS2Mac( &fgColor ) );
		SetForeColor( &fgColor );
		DrawTextBox( this, buf1, left, box.rectBottom - 5, kJustLeft, kFont );
		
		DTSColor urgency;
		if ( 0 == freeSlots )
			urgency.Set( 0xA000, 0x0,    0x0    );		// angry red
		else
		if ( freeSlots < 3 )
			urgency.Set( 0x8000, 0x4000, 0x4000 );		// concerned cayenne
		else
		if ( freeSlots < 5 )
			urgency.Set( 0x6000, 0x2000, 0x2000 );		// mellow maroon
		else
			urgency = fgColor;
		
		SetForeColor( &urgency );
		left += wid1;
		
		DrawTextBox( this, buf2, left, box.rectBottom - 5, kJustLeft, kFont );
		
		// reset things
		SetForeColor( &DTSColor::black );
		
		if ( buf1 )
			CFRelease( buf1 );
		if ( buf2 )
			CFRelease( buf2 );
		}
	
	// draw the subviews
	DTSView::DoDraw();
}


#if 0
/*
**	InvenContent::Click()
*/
bool
InvenContent::Click( const DTSPoint& /*start*/, ulong /*time*/, uint /*modifiers*/ )
{
	// do whatever you want with the click here...
	// basically we don't care about it.
	// the subviews will get a crack at it later.
	return false;
}
#endif


/*
**	InvenContent::BestSize()
**
**	given an arbitrary content size, return the nearest valid size (e.g. during live resize)
*/
DTSPoint
InvenContent::BestSize( DTSPoint s ) const
{
	int wd = s.ptH;
	int ht = s.ptV;
	
	// clamp to limits
	if ( wd < kMinInvWinWidth )
		wd = kMinInvWinWidth;
	if ( wd > kMaxInvWinWidth )
		wd = kMaxInvWinWidth;
	
	if ( ht < kMinInvWinHeight )
		ht = kMinInvWinHeight;
	if ( ht > kMaxInvWinHeight )
		ht = kMaxInvWinHeight;
	
	// ensure whole lines: round height down to nearest quantum
	int slop = Margins();				// height of fixed (non-list) placard section(s)
//	const int lh = kItemListHeight;		// or gInvenList.lvCellHeight (non-const)?
	ht = slop + kItemListHeight * ((ht - slop) / kItemListHeight);
	
	return DTSPoint( wd, ht );
}

#pragma mark -


#if USE_CFSTRING_COMPARE
/*
**	SameItemName()
**
**	are two item names the same (when compared via CFString-based APIs)?
*/
static inline bool
SameItemName( CFStringRef s1, CFStringRef s2 )
{
	return kCFCompareEqualTo == CFStringCompare( s1, s2,
									kItemNameCompareFlags );
}
#endif  // USE_CFSTRING_COMPARE


/*
**	ItemList::ItemList()
*/
ItemList::ItemList( DTSKeyID itmID, const ClientItem& data, bool equip ) :
		itemID( itmID ),
		itemData( data ),
#if USE_CFSTRING_COMPARE
		itemCFName( nullptr ),
#endif
		itemIndex( 0 ),
		itemQuantity( 1 ),
		itemCmdKey( 0 ),
		itemDirty( false ),
		itemEquipped( equip )
{
	itemCustomName[0] = '\0';
	
#if USE_CFSTRING_COMPARE
	// cache [the non-custom portion of] the name as a CFString
	const char * nm = data.itemName;
	
# ifdef MULTILINGUAL
	// FIXME! This doesn't null-terminate the found substring
	// at 'nm', which means CreateCFString() could slurp up too much.
	ExtractLangText( &nm, gRealLangID, nullptr );
# endif  // MULTILINGUAL
	
	itemCFName = CreateCFString( nm );
#endif  // USE_CFSTRING_COMPARE
}


/*
**	CapitalizeItemName()
**
**	Capitalize (not UPPER-CASE) the Incoming String
**	Using Rules Appropriate For Item Names
**	(Kind of Like This Comment :-)
*/
void
ItemList::CapitalizeItemName( char * name )
{
	bool up = true;
	for ( char * p = name; *p; ++p )
		{
		int ch = * (uchar *) p;
		if ( ' ' == ch )						// Giant Halberd
			up = true;
		else
		if ( '-' == ch )						// Second-Circle Pants
			up = true;
		else
		if ( up )
			{
			if ( isdigit( ch ) )				// 1st 2nd 3rd 4th
				up = false;
			else
			if ( not strncmp( p, "of ", 3 ) )	// don't capitalize "of "
				up = false;
			else
			if ( not strncmp( p, "the ", 4 ) )	// nor "the "
				up = false;
			else
			if ( not strncmp( p, "a ", 2 ) )	// nor "a "
				up = false;
			else
			if ( ch >= 'a' && ch <= 'z' )		// capitalize and skip to next space
				{
				* (uchar *) p = ch - ('a' - 'A');
#ifdef MULTILINGUAL
			 	// If text not in English, then capitalize only the first letter
				if ( gRealLangID != 0 )
					return;
#endif // MULTILINGUAL
				up = false;
				}
			else
			if ( ch >= 'A' && ch <= 'Z' )		// already capitalized; skip to next space
				up = false;
			}
		}
}


/*
**	GetCanonicalName()
**
**	returns the user-visible name of an item, including whatever's
**	necessary to distinguish individual template items,
**  especially the custom name if any
**  optionally capitalize the standard name, never capitalize the custom part
*/
void
ItemList::GetCanonicalName( char * outName,
			size_t nameLen, bool capitalize /* =false */ ) const
{
	char indexstr[ 10 ];
	size_t len;
	size_t custLen = strlen( itemCustomName );
	
#ifdef MULTILINGUAL
	// extract translation of item (if any)
	char myitemName[ kMaxItemNameLen ];
	const char * itempnt = itemData.itemName;
	int itemnameidx = ExtractLangText( &itempnt, gRealLangID, nullptr );
	StringCopySafe( myitemName, itempnt, itemnameidx + 1 );
	myitemName[ itemnameidx ] = '\0';
#else
	const char * myitemName = itemData.itemName;
#endif // MULTILINGUAL
	
	if ( itemData.itemFlags & kItemFlagData )
		{
		// only index
		if ( '\0' == itemCustomName[0] )
			{
			len = snprintf( indexstr, sizeof indexstr, " <#%d>", itemIndex + 1 );
			StringCopySafe( outName, myitemName, nameLen - len );
			if ( capitalize )
				CapitalizeItemName( outName );
			strlcat( outName, indexstr, nameLen );
			}
		
		// index and custom name
		else
			{
			len = snprintf( indexstr, sizeof indexstr, " <#%d: ", itemIndex + 1 );
			StringCopySafe( outName, myitemName, nameLen - len - custLen - 1 );
			// (that -1 is to ensure space for the '>', below)
			if ( capitalize )
				CapitalizeItemName( outName );
			strcat( outName, indexstr );
			strcat( outName, itemCustomName );
			strcat( outName, ">" );
			}
		}
	
	// only custom name (shouldn't happen for non-template items,
	// but we like to be thorough)
	else
	if ( itemCustomName[0] )
		{
		StringCopySafe( outName, myitemName, nameLen - custLen - 3 );	// "< >"
		if ( capitalize )
			CapitalizeItemName( outName );
		strcat( outName, " <" );
		strcat( outName, itemCustomName );
		strcat( outName, ">" );
		}
		
	// neither (most common case)
	else
		{
		StringCopySafe( outName, myitemName, nameLen );
		if ( capitalize )
			CapitalizeItemName( outName );
		}
}

#pragma mark -


/*
**	InvenList::DrawNode()
**
**	draw a single inventory item (a node in the listview)
*/
void
InvenList::DrawNode( int inIndex, const DTSRect& inRect, bool bActive )
{
	const ThemeFontID kFont = kThemeLabelFont;
	OSStatus err;
	
	// locate this item in list
	int top = inIndex;
	const ItemList * item = gRootInvItem;
	for (  ;  item && top;  item = item->linkNext, --top )
		;
	
	// paranoia -- keep static analyzer honest
	assert( item );
	
	// draw selected row, by first erasing to the regular or highlit color,
	// depending on whether this is the selected item
	if ( item == icSelect )
		{
		ThemeBrush brush = bActive ? kThemeBrushAlternatePrimaryHighlightColor
								   : kThemeBrushSecondaryHighlightColor;
		__Verify_noErr( SetThemeBackground( brush, 32, true ) );
		}
	else
		{
		__Verify_noErr( SetThemeBackground( kThemeBrushListViewBackground, 32, true ) );
		}
	Erase( &inRect );
	
	// normalize colors if need be
	if ( item == icSelect )
		__Verify_noErr( SetThemeBackground( kThemeBrushListViewBackground, 32, true ) );
	
	DTSRect dst = inRect;
	dst.rectBottom = dst.rectTop + lvCellHeight;
	dst.Inset( 1, 0 );
	
	// draw the item's wornPict icon
	const int kIconSize = 16;
	if ( const ImageCache * cache = CachePicture( item->itemData.itemWornPictID ) )
		{
		DTSRect box = dst;
		box.Offset( 1, 1 );
		box.Size( kIconSize, kIconSize );
		
		DTSRect src;
		cache->icImage.cliImage.GetBounds( &src );
		src.rectBottom = src.rectTop + cache->icHeight;
		
		Draw( &cache->icImage.cliImage, kImageTransparentQuality, &src, &box );
		}
	
	// get item's name, quantity, etc.
	char name[ 256 ];
	char canonical[ 256 ];
	char shortcut[ 8 ] = "1: ";		// prefix to show for items that have shortcut keys
	
	item->GetCanonicalName( canonical, sizeof canonical, true );
	if ( item->itemCmdKey )
		shortcut[0] = item->itemCmdKey;
	else
		shortcut[0] = '\0';		// i.e. no shortcut prefix at all
	
	if ( item->itemQuantity > 1 )
		snprintf( name, sizeof name, "%s%s (%d)", shortcut, canonical, item->itemQuantity );
	else
		snprintf( name, sizeof name, "%s%s", shortcut, canonical );
	CFStringRef name2 = CreateCFString( name );
	
	int avail = dst.rectRight - dst.rectLeft - 18 - 4;
	avail -= GetTextBoxWidth( this, name2, kFont );
	
	// special handling for items that are equipped
	int slotNameW = 0;	// text-width of slot name, or 0
	CFStringRef slotName = nullptr;
	
	if ( item->itemEquipped )
		{
		int slot = item->itemData.itemSlot;
		
		// easier this way
		const char * const itemSlots[] =
			{
			TXTCL_SLOTNAME_INVALID,		// "Invalid",
			TXTCL_SLOTNAME_UNKNOWN,		// "unknown",
			TXTCL_SLOTNAME_FOREHEAD,	// "forehead",
			TXTCL_SLOTNAME_NECK,		// "neck",
			TXTCL_SLOTNAME_SHOULDER,	// "shoulder",
			TXTCL_SLOTNAME_ARMS,		// "arms",
			TXTCL_SLOTNAME_GLOVES,		// "gloves",
			TXTCL_SLOTNAME_FINGER,		// "finger",
			TXTCL_SLOTNAME_COAT,		// "coat",
			TXTCL_SLOTNAME_CLOAK,		// "cloak",
			TXTCL_SLOTNAME_TORSO,		// "torso",
			TXTCL_SLOTNAME_WAIST,		// "waist",
			TXTCL_SLOTNAME_LEGS,		// "legs",
			TXTCL_SLOTNAME_FEET,		// "feet",
			TXTCL_SLOTNAME_RIGHT,		// "right",
			TXTCL_SLOTNAME_LEFT,		// "left",
			TXTCL_SLOTNAME_HANDS,		// "hands",
			TXTCL_SLOTNAME_HEAD			// "head"
			};
		
		// quick & dirty display of what slot it's in
		// eventually, draw a mannequin or something
		// and below that, list unequipped backpack contents.
		int nameIndex = (slot >= kItemSlotFirstReal && slot <= kItemSlotLastReal) ?
							slot : kItemSlotNotWearable;
		slotName = CFStringCreateFormatted( CFSTR("[%s]"), itemSlots[ nameIndex ] );
		slotNameW = GetTextBoxWidth( this, slotName, kFont ) + 2;
		}
	
	ThemeTextColor textColor = kThemeTextColorListView;
	if ( bActive && item == icSelect )
		textColor = kThemeTextColorWhite;
	__Verify_noErr( SetThemeTextColor( textColor, 32, true ) );
	
	//
	// can't go directly to DrawTextBox() here because that won't do underlining,
	// and we're going to need to measure the text vertically as well as horizontally
	//
	if ( name2 )
		{
		ThemeDrawState state = bActive ? kThemeStateActive : kThemeStateInactive;
		DTSRect r = dst;
		r.rectLeft += kIconSize + 2;
		r.rectTop += 2;
		r.rectRight -= 1;
//		r.rectBottom += 0;			// no-op
		DrawThemeTextBox( name2, kFont, state, false, DTS2Mac( &r ), teJustLeft, nullptr );
		
		// we need to handle underlining separately, since DTTB doesn't
		if ( item->itemEquipped )
			{
			DTSPoint dimens( 0, 0 );
			SInt16 baseline;
			err = GetThemeTextDimensions( name2, kFont, state, false,
						DTS2Mac( &dimens ), &baseline );
			if ( noErr == err )
				{
				// fetch correct color to use for the underline
				ThemeBrush tb = (kThemeTextColorWhite == textColor) ?
								kThemeBrushWhite : kThemeBrushBlack;
				__Verify_noErr( SetThemePen( tb, 32, true ) );
				
				DTSLine underliner;
				underliner.Set( r.rectLeft,
								r.rectTop + dimens.ptV,
								r.rectLeft + dimens.ptH,
								r.rectTop + dimens.ptV );
				Draw( &underliner );
				}
			}
		CFRelease( name2 );
		}
	
	// draw slot name right-justified, if there is one, and there's room for it
	if ( slotName  &&  avail > slotNameW )
		{
		DrawTextBox( this, slotName, dst.rectRight - 2, dst.rectBottom - 5,
				kJustRight, kFont );
		}
	
	if ( slotName )
		CFRelease( slotName );
}


/*
**	InvenList::ClickNode()
**
**	select and drag the items
*/
bool
InvenList::ClickNode( int inIndex, const DTSPoint& /*start*/, ulong time, uint mods )
{
	int top = inIndex;
	ItemList * select = icSelect;
	
	// locate this item in the list
	ItemList * item = gRootInvItem;
	for (  ;  item && top;  item = item->linkNext, --top )
		;
	
	// select it (locally)
	InvenWindow * iWin = static_cast<InvenWindow *>( gInvenWindow );
	if ( select != item )
		{
		icSelect = item;
		iWin->UpdateInvenMenu();
		
		// redraw everything
		Update();
		}
	else
	if ( mods & kKeyModMenu )
		{
		icSelect = nullptr;
		iWin->UpdateInvenMenu();
		
		// redraw everything
		Update();
		}
	
	char buff[ 256 ];
	
	// select it (on server; if different)
	if ( icSelect != select )
		{
		DTSKeyID newID = 0;
		int itmIndex = 0;
		if ( icSelect )
			{
			assert( item );
			newID = item->itemID;
			itmIndex = item->itemIndex;
			}
		
		snprintf( buff, sizeof buff, "\\BE-SELECT %d %d", (int) newID, itmIndex );
		SendText( buff, true );
		}
	
	// handle further semantics
	if ( icSelect )
		{
		assert( item );	// keep static analyzer on track
		
		int delta = time - lvLastClickTime;
		bool doubleClick = false;
		if ( select == icSelect
		&&	 delta <= (int) ::GetDblTime() )
			{
			doubleClick = true;
			}
		
		// plain click selects
		if ( not doubleClick
		&&	 not mods )
			{
			// we're done
			return true;
			}
		
		if ( doubleClick )
			{
			// shift-double-click uses
			if ( mods & kKeyModShift )
				{
				snprintf( buff, sizeof buff, "\\USEITEM %d", (int) item->itemID );
				SendText( buff, true );
				}
			else
			// other double-clicks equip (unless equipped already)
			if ( not item->itemEquipped )
				SendInvCmd( item->itemID, item->itemIndex + 1 );
			else
			// double-click equipped item unequips it
			if ( item->itemEquipped )
				{
				snprintf( buff, sizeof buff, "\\UNEQUIP %d", (int) item->itemID );
				SendText( buff, true );
				}
			}
		// other types of click aren't defined (yet)
		}
	
	return true;
}


/*
**	InvenList::KeyStroke()
**
**	handle assigning shortcut keys
*/
bool
InvenList::KeyStroke( int ch, uint modifiers )
{
	if ( icSelect
	&&	 ( isdigit(ch) || ' ' == ch ) )
		{
		// clear other items of this key
		for ( ItemList * item = gRootInvItem;  item;  item = item->linkNext )
			{
			if ( item->itemCmdKey == ch )
				{
				item->itemCmdKey = 0;
				item->itemDirty = true;
				}
			}
		
		// Assign this key to the selected item
		icSelect->itemCmdKey = ch == ' ' ? 0 : ch;
		icSelect->itemDirty = true;
		
		InventorySave( false );
		
		SortInventoryList();
		
		// Scroll it into view
		int idx = 0;
		for ( const ItemList * item = gRootInvItem;  item;  item = item->linkNext )
			{
			if ( item == icSelect )
				break;
			++idx;
			}
		ShowNode( idx );
		Update();
		return true;
		}
	
	return ListView::KeyStroke( ch, modifiers );
}

#pragma mark -


/*
**	ResetInventory()
**
**	reset the inventory list
*/
void
ResetInventory()
{
	gInvenCmd = kPlyInvCmdNone;
	
	gLeftPictID = gRightPictID = 0;
	gLeftItemID = gRightItemID = 0;
	
	ItemList::DeleteLinkedList( gRootInvItem );
	
	gInvenList.SetSelection( nullptr );
	gInvenList.lvNumNodes = 0;
	if ( InvenWindow * win = static_cast<InvenWindow *>( gInvenWindow ) )
		win->UpdateInvenMenu();
}


/*
**	InventoryItemCompare()
**
**	comparison callback function for sorting the inventory list, via qsort()
*/
static int
InventoryItemCompare( const void * v1, const void * v2 )
{
	// remember that we're sorting an array of pointers!
	const ItemList * i1 = * static_cast<const ItemListPtr *>( v1 );
	const ItemList * i2 = * static_cast<const ItemListPtr *>( v2 );
	
	// prioritize command keys first --
	// items w/o cmd-keys sort after all those that have 'em
	int key1 = isdigit( i1->itemCmdKey ) ? i1->itemCmdKey : 100;	// any value > '9'
	int key2 = isdigit( i2->itemCmdKey ) ? i2->itemCmdKey : 100;
	
	int result = key1 - key2;
	if ( result )
		return result;
	
	// next, alphabetize the names (official ones only; ignoring any custom labels)
	// ideally we want to ignore both case and diacritics.
	// There's no easy portable way to do that.
	// On Macs, we can use the OS's services, e.g. from <StringCompare.h>
	// ... or (better yet) from <CFString.h>
	
#if USE_CFSTRING_COMPARE
	if ( i1->itemCFName && i2->itemCFName )
		result = CFStringCompare( i1->itemCFName,
								  i2->itemCFName, kItemNameCompareFlags );
	
#else  // ! USE_CFSTRING_COMPARE

	const char * n1 = i1->itemData.itemName;
	const char * n2 = i2->itemData.itemName;
	
# ifdef MULTILINGUAL
	ExtractLangText( &n1, gRealLangID, nullptr );
	ExtractLangText( &n2, gRealLangID, nullptr );
# endif
	
# if USE_MACOS_STRING_COMPARE

	result = ::CompareText( n1, n2, strlen(n1), strlen(n2), nullptr );
	
# else  // neither OSX nor classic MacOS
	
	// this is the old way, and is locale-sensitive. Net result, diacritics will
	// NOT be collapsed, and case may or may not be handled correctly for hi-bit
	// characters ("a-acute" may sort very, very far away from "a").
	do
		{
		result += toupper( * (const uchar *) n1 ) - toupper( * (const uchar *) n2 );
		}
	while ( *n1++ && *n2++ && not result );
# endif  // ! MACOS_STRING_COMPARE
#endif  // ! USE_CFSTRING_COMPARE
	
	if ( result )
		return result;
	
	// finally, if names (& cmd-keys) match, order by index
	return i1->itemIndex - i2->itemIndex;
}


/*
**	SortInventoryList()
**
**	arrange the linked list of items into a suitable order
*/
bool
SortInventoryList()
{
	int count = CountInventoryItems();
	if ( count <= 1 )
		return false;
	
	if ( ItemListPtr * table = NEW_TAG("ItemListSorted") ItemListPtr[ count ] )
		{
		ItemList * it = gRootInvItem;
		
		// init table
		for ( int i = 0; it; it = it->linkNext, ++i )
			table[i] = it;
		
		qsort( table, count, sizeof table[0], InventoryItemCompare );
		
		// rebuild links
		gRootInvItem = table[0];
		for ( int i = 0; i < count; ++i )
			table[i]->linkNext = table[ i + 1 ];
		table[ count - 1 ]->linkNext = 0;
		
		delete[] table;
		}
	return true;
}


#if ITEM_PREFS_IN_PLIST
/*
**	CreatePrefKeyForItem()
**
**	Given a specific inventory item, return the key used to access its hotkey
**	from the hotkey dictionary
*/
static CFStringRef
CreatePrefKeyForItem( const ItemList * it )
{
	// template items have to encode their indexes into the file key ID
	DTSKeyID theID;
	if ( it->itemData.itemFlags & kItemFlagData )
		theID = MakeLong( it->itemID, it->itemIndex );
	else
		theID = it->itemID;
	
	CFStringRef key = CFStringCreateFormatted( CFSTR("%d"), theID );
	return key;
}
#endif  // ITEM_PREFS_IN_PLIST



/*
**	InventorySave()
**
**	Saves dirty items, all in one place
*/
void
InventorySave( bool inAll )
{
	// remove any previous occurrences
	for ( ItemList * it = gRootInvItem; it;  it = it->linkNext )
		{
		if ( it->itemDirty || inAll )
			{
#if ITEM_PREFS_IN_PLIST
			if ( CFStringRef key = CreatePrefKeyForItem( it ) )
				{
				int shortcut = it->itemCmdKey;
				if ( shortcut )
					{
					shortcut -= '0';	// remap '0'..'9' to 0..9
					
					if ( CFNumberRef val = CFNumberCreate( kCFAllocatorDefault,
											kCFNumberIntType, &shortcut ) )
						{
						CFDictionarySetValue( sItemShortcutKeys, key, val );
						CFRelease( val );
						}
					}
				else
					{
					// no assigned shortcut
					CFDictionaryRemoveValue( sItemShortcutKeys, key );
					}
				
				CFRelease( key );
				}
#endif  // ITEM_PREFS_IN_PLIST
			
			SItemPrefs prefs;
			prefs.itemCmdKey = it->itemCmdKey;
			
			DTSKeyID theID;
			if ( it->itemData.itemFlags & kItemFlagData )
				theID = MakeLong( it->itemID, it->itemIndex );
			else
				theID = it->itemID;
			
			DTSError result = gClientPrefsFile.Write( kTypeItemPrefs, theID,
				&prefs, sizeof prefs );
			if ( noErr == result )
				it->itemDirty = false;
			}
		}
}


#if ITEM_PREFS_IN_PLIST
/*
**	SaveInventoryShortcutKeys
*/
void
SaveInventoryShortcutKeys()
{
	Prefs::Set( pref_InventoryShortcut, sItemShortcutKeys );
}


/*
**	GetInventoryShortcutKeys()
*/
void
GetInventoryShortcutKeys()
{
	if ( CFTypeRef tr = Prefs::Copy( pref_InventoryShortcut ) )
		{
		if ( CFDictionaryGetTypeID() == CFGetTypeID( tr ) )
			{
			if ( sItemShortcutKeys )
				CFRelease( sItemShortcutKeys );
			
			sItemShortcutKeys = CFDictionaryCreateMutableCopy( kCFAllocatorDefault,
									0, static_cast<CFDictionaryRef> ( tr ) );
			CFRetain( sItemShortcutKeys );
			}
		CFRelease( tr );
		}
}
#endif  // ITEM_PREFS_IN_PLIST


/*
**	InventoryShortcut()
**
**	Check if a keystroke is an item-related cmd-key,
**	and equip the appropriate thing when needed
*/
bool
InventoryShortcut( int ch, uint modifiers )
{
	// we're looking for Cmd-0 thru Cmd-9
	if ( (modifiers & kKeyModMenu) != kKeyModMenu )
		return false;
	
	int c = ch & 0x0FF;
	for ( const ItemList * it = gRootInvItem;  it;  it = it->linkNext )
		{
		if ( it->itemCmdKey == c )
			{
			ClearInvCmd();
			
			if ( it->itemData.itemFlags & kItemFlagData )
				SendInvCmd( it->itemID, it->itemIndex + 1 );
			else
				SendInvCmd( it->itemID );
			
			break;
			}
		}
	
	if ( isdigit(c) )
		return true;		// we handled it
	
	return false;
}


/*
**	CountInventoryItems()
**
**	how many ya got? (maybe counting duplicates)
*/
int
CountInventoryItems( bool wantDups )
{
	int count = 0;
	for ( const ItemList * it = gRootInvItem;  it;  it = it->linkNext )
		{
		// either add all of these to the count, or just 1
		count += wantDups ? it->itemQuantity : 1;
		}
	return count;
}


/*
**	SetHandIcons()
**
**	adjust the hand-icon globals after adding an equipped item
*/
static void
SetHandIcons( DTSKeyID id, const ClientItem& data )
{
	if ( kItemSlotRightHand == data.itemSlot )
		{
		gRightItemID = id;
		gRightPictID = data.itemRightHandPictID;
		}
	else
	if ( kItemSlotLeftHand == data.itemSlot )
		{
		gLeftItemID = id;
		gLeftPictID = data.itemLeftHandPictID;
		}
}


#if DTS_LITTLE_ENDIAN

// helper
# define B2NF( x )  x = BigToNativeEndian( x );

/*
**	BigToNativeEndian( ClientItem * )
**
**	the usual byte-flipping rigmarole
*/
static void
BigToNativeEndian( ClientItem * itm )
{
	B2NF( itm->itemFlags );
	B2NF( itm->itemSlot );
	B2NF( itm->itemRightHandPictID );
	B2NF( itm->itemLeftHandPictID );
	B2NF( itm->itemWornPictID );
}

# undef B2NF
#endif  // DTS_LITTLE_ENDIAN


/*
**	AddToInventory()
**
**	add an item to the inventory list
*/
void
AddToInventory( DTSKeyID id, int idx, bool equipped )
{
	// already got one of these?
	// (ignore idx in the search -- for non-template items it's irrelevant
	// and template items get special handling)
	ItemList * item = FindItemByIDAndIndex( id );
	if ( item
	&&	 (item->itemData.itemFlags & kItemFlagData) == 0 )
		{
		++item->itemQuantity;
		
		if ( equipped )
			{
			// this way, when quantity > 1, then itemEquipped is true if at least
			// one instance of this item is equipped. But if no instances are equipped,
			// (according to the server's bitmap) then itemEquipped will remain false.
			item->itemEquipped = true;
			
			// in the same vein: if the first instance was in a hand slot, but wasn't equipped,
			// whereas a subsequent instance WAS equipped, it's necessary to update the
			// hand icons now.
			SetHandIcons( id, item->itemData );
			}
		
		return;
		}
	
	// create a node to hold it
	ClientItem data;
	DTSError result = gClientImagesFile.Read( kTypeClientItem, id, &data, sizeof data );
	if ( result != noErr )
		{
		ShowMessage( "Failed to load data for item %d result %d.", (int) id, (int) result );
		return;
		}
	
#if DTS_LITTLE_ENDIAN
	BigToNativeEndian( &data );
#endif
	
	item = NEW_TAG("ItemList") ItemList( id, data, equipped );
	if ( not item )
		{
		ShowMessage( "Failed to allocate memory for an inventory item." );
		return;
		}
	
	if ( data.itemFlags & kItemFlagData )
		{
		// when adding a single template item, its co-instances must renumber
		if ( idx != -1 )
			{
			RenumberItemIndex( id, idx, true );	// renumber upward
			item->itemIndex = idx;
			}
		else
			{
			// but when handling kInvCmdFull, we have to assign indexes sequentially ourselves
			idx = 0;
			// count how many we already have
			for ( const ItemList * temp = gRootInvItem;  temp;  temp = temp->linkNext )
				{
				if ( temp->itemID == id )
					++idx;
				}
			// that becomes the new index
			item->itemIndex = idx;
			}
		}
	
	// special handling for equipped items in hand slots
	if ( equipped )
		SetHandIcons( id, data );
	
	// inspect prefs for this item
	SItemPrefs prefs;
	prefs.itemCmdKey = 0;
	
#if ITEM_PREFS_IN_PLIST
	char savedKey = 0;
	if ( CFStringRef key = CreatePrefKeyForItem( item ) )
		{
		if ( CFTypeRef val = CFDictionaryGetValue( sItemShortcutKeys, key ) )
			{
			if ( CFNumberGetTypeID() == CFGetTypeID( val ) )
				{
				CFNumberRef nr = static_cast<CFNumberRef>( val );
				int keyval = 0;
				CFNumberGetValue( nr, kCFNumberIntType, &keyval );
				savedKey = keyval + '0';
				}
			}
		
		CFRelease( key );
		}
	
	item->itemCmdKey = savedKey;
	if ( savedKey != prefs.itemCmdKey )
		item->itemDirty = true;
	
	// Clear the hotkey if it had been reassigned
	for ( const ItemList * it = gRootInvItem; item->itemCmdKey && it; it = it->linkNext )
		if ( it->itemCmdKey == item->itemCmdKey
		&&	 it->itemIndex  == item->itemIndex )
			{
			item->itemCmdKey = 0;
			item->itemDirty = true;
			}

#else  // ! ITEM_PREFS_IN_PLIST
	
	// template items have to encode their indexes into the file key ID
	DTSKeyID theID;
	if ( data.itemFlags & kItemFlagData )
		theID = MakeLong( id, idx );
	else
		theID = id;
	
	result = gClientPrefsFile.Read( kTypeItemPrefs, theID, &prefs, sizeof prefs );
	if ( noErr == result )
		{
		char savedKey = prefs.itemCmdKey;
		if ( savedKey < '0' || savedKey > '9' )
			savedKey = 0;
		
		item->itemCmdKey = savedKey;
		if ( savedKey != prefs.itemCmdKey )
			item->itemDirty = true;
		
		// Clear the hotkey if it had been reassigned
		for ( const ItemList * it = gRootInvItem; it && item->itemCmdKey;
			  it = it->linkNext )
			{
			if ( it->itemCmdKey == item->itemCmdKey
			&&	 it->itemIndex  == item->itemIndex )
				{
				item->itemCmdKey = 0;
				item->itemDirty = true;
				}
			}
		}
#endif  // ITEM_PREFS_IN_PLIST
	
	if ( equipped )
		BumpItemsFromSlot( id, data.itemSlot, item->itemIndex );
	
	item->InstallFirst( gRootInvItem );
	
	SortInventoryList();		// this might be a waste of time during kInvCmdFull
	
	InventorySave( false );
	
	if ( InvenWindow * win = static_cast<InvenWindow *>( gInvenWindow ) )
		{
		++gInvenList.lvNumNodes;
		int maxHt = win->iwContent.Margins() +
					 gInvenList.lvNumNodes * gInvenList.lvCellHeight;
		if ( maxHt > kMaxInvWinHeight )
			maxHt = kMaxInvWinHeight;
		win->SetZoomSize( kMaxInvWinWidth, maxHt );
		}
}


/*
**	InventorySetCustomName()
**
**	set custom name for one item
*/
void
InventorySetCustomName( DTSKeyID id, int idx, const char * text )
{
	// find the item; set its name
	if ( ItemList * item = FindItemByIDAndIndex( id, idx ) )
		{
		if ( strlen( text ) > sizeof item->itemCustomName )
			{
			// truncate and append "..."
			StringCopySafe( item->itemCustomName, text, sizeof item->itemCustomName - 4 );
			strlcat( item->itemCustomName, "...", sizeof item->itemCustomName );
			}
		else
			StringCopySafe( item->itemCustomName, text, sizeof item->itemCustomName );
		}
}


/*
**	BumpItemsFromSlot()
**
**	unequip all items that are in this slot
*/
void
BumpItemsFromSlot( DTSKeyID id, int slot, int itemIndex )
{
	for ( ItemList * item = gRootInvItem;  item;  item = item->linkNext )
		{
		if ( item->itemData.itemSlot == slot
		&&	 (item->itemID != id  || item->itemIndex != itemIndex) )
			{
			item->itemEquipped = false;
			}
		}
}


/*
**	InventoryEquipItem()
**
**	change equip status of one item
*/
void
InventoryEquipItem( DTSKeyID itemID, bool equip, int idx )
{
	if ( ItemList * item = FindItemByIDAndIndex( itemID, idx ) )
		{
		// equip it
		item->itemEquipped = equip;
		
		// the hands may have changed
		int slot = item->itemData.itemSlot;
		if ( kItemSlotRightHand == slot )
			{
			gRightItemID = equip ? itemID : 0;
			gRightPictID = equip ? item->itemData.itemRightHandPictID : 0;
			}
		else
		if ( kItemSlotLeftHand == slot )
			{
			gLeftItemID = equip ? itemID : 0;
			gLeftPictID = equip ? item->itemData.itemLeftHandPictID : 0;
			}
		
		if ( equip )
			BumpItemsFromSlot( itemID, slot, idx );
		
//		UpdateInventoryWindow();
		}
}


/*
**	InventoryRemoveItem()
**
**	delete one item
*/
void
InventoryRemoveItem( DTSKeyID id, int idx )
{
	if ( ItemList * item = FindItemByIDAndIndex( id, idx ) )
		{
		// if we had more than one, it's easy
		if ( --item->itemQuantity > 0 )
			return;
		
		// might have been holding it
		int slot = item->itemData.itemSlot;
		if ( item->itemEquipped )
			{
			if ( kItemSlotRightHand == slot )
				{
				gRightItemID = 0;
				gRightPictID = 0;
				}
			else
			if ( kItemSlotLeftHand == slot )
				{
				gLeftItemID = 0;
				gLeftPictID = 0;
				}
			}
		
		// template data items need to be renumbered downward
		if ( item->itemData.itemFlags & kItemFlagData )
			RenumberItemIndex( id, idx, false );
		
		// get rid of it
		item->Remove( gRootInvItem );
		delete item;
		
		// tell the listview
		--gInvenList.lvNumNodes;
		if ( gInvenList.Selected() == item )
			gInvenList.SetSelection( nullptr );
		}
}


/*
**	InventorySaveSelection()
**
**	remember the selection to restore after a reset
*/
long
InventorySaveSelection()
{
	if ( ItemList * sel = gInvenList.Selected() )
		return MakeLong( sel->itemID, sel->itemIndex );
	
	return 0;
}


/*
**	InventoryRestoreSelection()
*/
void
InventoryRestoreSelection( long id )
{
	if ( 0 == id )
		return;
	int itemID = (id >> 16) & 0x0FFFF;
	int idx = id & 0x0FFFF;
	
	if ( ItemList * item = FindItemByIDAndIndex( itemID, idx ) )
		gInvenList.SetSelection( item );
}


/*
**	FindItemByIDAndIndex()
**
**	find an item by id (and possibly index)
*/
ItemList *
FindItemByIDAndIndex( DTSKeyID id, int idx )
{
	for ( ItemList * item = gRootInvItem;  item;  item = item->linkNext )
		{
		if ( item->itemID == id
		&&	 item->itemIndex == idx )
			{
			return item;
			}
		}
	return nullptr;
}


/*
**	GetInvItemName()
**
**	return the name of the specified item
*/
size_t
GetInvItemName( DTSKeyID id, char * dst, size_t maxlen )
{
	if ( dst && maxlen )
		*dst = '\0';
	else
		return 0;	// no room for name!
	
	if ( const ItemList * item = FindItemByIDAndIndex( id ) )
		item->GetCanonicalName( dst, maxlen );
	else
		{
		// "Nothing"
		StringCopySafe( dst, _(TXTCL_ITEMNAME_NOTHING), maxlen );
		}
	
	return strlen( dst );
}


/*
**	GetInvItemNameBySlot()
**
**	return the name of the equipped item currently in that slot
*/
void
GetInvItemNameBySlot( int slot, char * dst, size_t maxlen )
{
	if ( dst && maxlen )
		*dst = '\0';
	else
		return;		// no room!
	
	const ItemList * item;
	for ( item = gRootInvItem;  item;  item = item->linkNext )
		{
		if ( item->itemEquipped
		&&	 item->itemData.itemSlot == slot )
			{
			break;
			}
		}
	if ( item )
		item->GetCanonicalName( dst, maxlen );
	else
		{
		// "Nothing"
		StringCopySafe( dst, _(TXTCL_ITEMNAME_NOTHING), maxlen );
		}
}


/*
**	GetSelectedItemName()
**
**	return the name of the selected item
*/
void
GetSelectedItemName( char * dst, size_t maxlen )
{
	if ( dst && maxlen )
		*dst = '\0';
	else
		return;
	
	if ( const ItemList * sel = gInvenList.Selected() )
		sel->GetCanonicalName( dst, maxlen );
	else
		{
		// "Nothing"
		StringCopySafe( dst, _(TXTCL_ITEMNAME_NOTHING), maxlen );
		}
}


/*
**	FindItemByName()
**
**	attempt to find an inventory item whose canonical name matches 'name'
*/
static ItemList *
FindItemByName( const char * name )
{
	if ( not name )
		return nullptr;
	
#if USE_CFSTRING_COMPARE
	CFStringRef cname = CreateCFString( name );
	if ( not cname )
		return nullptr;
#elif USE_MACOS_STRING_COMPARE
	size_t nameLen = strlen( name );
#endif
	
	for ( ItemList * item = gRootInvItem; item; item = item->linkNext )
		{
		char test[ 256 ];
		item->GetCanonicalName( test, sizeof test );
		
#if USE_CFSTRING_COMPARE
		if ( CFStringRef ctest = CreateCFString( test ) )
			{
			if ( SameItemName( cname, ctest ) )
				{
				CFRelease( ctest );
				CFRelease( cname );
				
				return item;
				}
			
			CFRelease( ctest );
			}
#elif USE_MACOS_STRING_COMPARE
		if ( 0 == ::IdenticalText( name, test, nameLen, strlen(test), nullptr ) )
			return item;
#else
		if ( 0 == strcasecmp( name, test ) )
			return item;
#endif  // USE_CFSTRING_COMPARE / USE_MACOS_STRING_COMPARE
		}
	
#if USE_CFSTRING_COMPARE
	CFRelease( cname );
#endif
	
	return nullptr;
}


#ifdef MULTILINGUAL
/*
**	GetInvItemByShortTag()
**
**	This looks for a known item short tag (e.g. %lfcy)
**	and returns the full translated itemname that matches.
**	Used by the macro engine to allow macros that are language independent.
**	(Stuff like @my.left_item >= "%lfcy" works that way in every language.)
*/
void
GetInvItemByShortTag( const char * shorttag, char * dst, size_t maxlen )
{
	if ( shorttag && dst && maxlen )
		*dst = '\0';
	else
		return;
	
	char test[ 256 + 3 ];
	char test2[ 256 + 4 ];
	
	// match for item public name encoding
	// (e.g. "roguewood club%l1Knueppel%l9club")
	// language 9 is reserved for the short tags
	snprintf( test, sizeof test, "%%l9%s", shorttag + 1 );
	snprintf( test2, sizeof test2, "%%ln9%s", shorttag + 1 );
	
	const ItemList * item;
	for ( item = gRootInvItem; item; item = item->linkNext )
		{
		const char * itempnt = item->itemData.itemName;
		if ( strstr( itempnt, test ) || strstr( itempnt, test2 ) )
			{
			// found short tag within that item
			item->GetCanonicalName( dst, maxlen );
			break;
			}
		}
	
	// not found? return original shorttag
	if ( not item )
		StringCopySafe( dst, shorttag, maxlen );
}
#endif // MULTILINGUAL


/*
**	RenumberItemIndex()
**
**	when adding or removing a template item, this routine
**	renumbers the other instances as necessary
*/
void
RenumberItemIndex( DTSKeyID itemID, int idx, bool upward )
{
	// when adding   new index N, any existing indexes N..Z   must go up   by 1
	// when removing old index N, any existing indexes N+1..Z must go down by 1
	int pivot = idx;
	int delta = +1;
	if ( not upward )
		{
		pivot = idx + 1;
		delta = -1;
		}
	
	for ( ItemList * item = gRootInvItem;  item;  item = item->linkNext )
		{
		if ( item->itemID == itemID )
			if ( item->itemIndex >= pivot )
				item->itemIndex += delta;
		}
}


/*
**	ResolveItemName()
**
**	Find a partial item name and return the whole one.
*/
bool
ResolveItemName( SafeString * name, bool showError )
{
	char cName[ 256 ], testName[ 256 ];
	
#if USE_CFSTRING_COMPARE
	CFStringRef cfname = CreateCFString( name->Get() );
#else
	// make an uppercase copy of the input to speed comparison
	StringCopySafe( cName, name->Get(), sizeof cName );
	
	// Once we have a MacRoman-aware version of strstr(), this can go away
	StringToUpper( cName );
#endif  // not USE_CFSTRING_COMPARE
	
#if USE_MACOS_STRING_COMPARE
	size_t nameLen = strlen( cName );
#endif
	
	// First look for exact matches
	for ( const ItemList * item = gRootInvItem; item; item = item->linkNext )
		{
		item->GetCanonicalName( testName, sizeof testName );
		
#if USE_CFSTRING_COMPARE
		CFStringRef cfTest = CreateCFString( testName );
		if ( not cfTest )
			continue;
		
		if ( SameItemName( cfname, cfTest ) )
#elif USE_MACOS_STRING_COMPARE
		if ( 0 == ::IdenticalText( cName, testName, nameLen, strlen(testName), nullptr ) )
#else
//		StringToUpper( testName );
		if ( 0 == strcasecmp( cName, testName ) )
#endif  // USE_CFSTRING_COMPARE / USE_MACOS_STRING_COMPARE
			{
			// found it! De-capitalize and return it
			item->GetCanonicalName( cName, sizeof cName );	 // ...and this can vanish
			name->Set( cName );
			
#if USE_CFSTRING_COMPARE
			CFRelease( cfTest );
			CFRelease( cfname );
#endif
			return true;
			}
#if USE_CFSTRING_COMPARE
		CFRelease( cfTest );
#endif
		}
	
	// Then look for partial matches
	for ( const ItemList * item = gRootInvItem; item; item = item->linkNext )
		{
		item->GetCanonicalName( testName, sizeof testName );
	
#if USE_CFSTRING_COMPARE
		CFStringRef cfTest = CreateCFString( testName );
		if ( not cfTest )
			continue;
		
		CFRange found = CFStringFind( cfTest, cfname, kItemNameCompareFlags );
		CFRelease( cfTest );
		if ( kCFNotFound != found.location )
		
//#elif USE_MACOS_STRING_COMPARE
		// FIXME: need to provide some form of strstr()
		// in the meantime, just use the old method
#else
		if ( strcasestr( testName, cName ) )
#endif  // USE_CFSTRING_COMPARE
			{
			item->GetCanonicalName( cName, sizeof cName );	// ... and this
			name->Set( cName );
			
#if USE_CFSTRING_COMPARE
			CFRelease( cfname );
#endif
			return true;
			}
		}
	
	// are we giving user feedback?
	if ( showError )
		{
		SafeString errMsg;
		// "No item named '%s' found in the item list."
		errMsg.Format( _(TXTCL_NO_SUCH_ITEM_IN_LIST), name->Get() );
		ShowInfoText( errMsg.Get() );
		}
	
#if USE_CFSTRING_COMPARE
	CFRelease( cfname );
#endif
	
	return false;
}


/*
**	EquipNamedItem()
**
*/
DTSError
EquipNamedItem( const char * name )
{
	// abort if there's a pending command
	if ( gInvenCmd != kInvCmdNone )
		return -1;
	
	// try to find the named item
	const ItemList * item = FindItemByName( name );
	
	// fail if there's no such item
	if ( not item )
		return -2;
	
	SendInvCmd( item->itemID, item->itemIndex + 1 );
	return noErr;
}


/*
**	SelectNamedItem()
**
*/
DTSError
SelectNamedItem( const char * name )
{
	// check for errors
	if ( not gInvenWindow )
		return -1;
	
	// try to find the named item
	ItemList * item = FindItemByName( name );
	
	// remember the current selection for comparison
	const ItemList * curSelect = gInvenList.Selected();
	
	// already selected -- soft error
	if ( curSelect == item )
		return +1;
	
	gInvenList.SetSelection( item );
	
	// scroll it into view
	int idx = 0;
	for ( const ItemList * test = gRootInvItem;  test;  test = test->linkNext )
		{
		if ( item == test )
			{
			gInvenList.ShowNode( idx );
			break;
			}
		++idx;
		}
	UpdateInventoryWindow();
	
	// inform the server
	char buff[ 256 ];
	DTSKeyID newID = item ? item->itemID  : 0;
	int newIndex = item ? item->itemIndex : 0;
	snprintf( buff, sizeof buff, "\\BE-SELECT %d %d", (int) newID, newIndex );
	SendText( buff, true );
	
	return noErr;
}


/*
**	SendInvCmd()
**
** Used by Commands to empty the hands
**
*/
DTSError
SendInvCmd( int inCmd, int idx /* =0 */ )
{
	// abort if there's a pending command
	if ( gInvenCmd != kInvCmdNone )
		return -1;
	
	gInvenCmd = inCmd;
	
	switch ( inCmd )
		{
		case kPlyInvCmdNone:
			break;
		
		case kPlyInvCmdEmptyLeft:
			gCmdList->SendCmd( false, "/unequip left" );
			break;
		
		case kPlyInvCmdEmptyRight:
			gCmdList->SendCmd( false, "/unequip right" );
			break;
		
		default:
			{
			char buff[ 256 ];
			if ( idx )
				snprintf( buff, sizeof buff, "/equip %d %d", inCmd, idx );
			else
				snprintf( buff, sizeof buff, "/equip %d", inCmd );
			gCmdList->SendCmd( false, buff );
			gInvenCmd = kInvCmdNone;
			} break;
		}
	
	return noErr;
}


/*
**	ClickSlotObject()
**
**	handle a click in the given slot
**	cycle through all of the objects appropriate for that slot
**	including the empty hand
**	(so far only supports hands)
*/
void
ClickSlotObject( int slot )
{
	// temporary protection
	if ( slot != kItemSlotRightHand
	&&	 slot != kItemSlotLeftHand )
		{
#ifdef DEBUG_VERSION
		ShowMessage( "ClickSlotObject() called with slot %d.", slot );
#endif
		return;
		}
	
	// check the pending inventory command
	int curItemID = gInvenCmd;
	
	// modify the ID if there is no pending command
	if ( kPlyInvCmdNone == curItemID )
		curItemID = (slot == kItemSlotRightHand) ? gRightItemID : gLeftItemID;
	
	// find the very first item appropriate for the slot
	// and find the first appropriate item for that slot
	// after the item currently there
	const ItemList * curItem   = nullptr;
	const ItemList * nextItem  = nullptr;
	const ItemList * firstItem = nullptr;
	int idx = 0;
	
	for ( const ItemList * item = gRootInvItem;  item;  item = item->linkNext )
		{
		// items that can fit the given slot
		if ( item->itemData.itemSlot == slot )
			{
			// remember the first item
			if ( not firstItem )
				firstItem = item;
			// stop when we've found the next item
			if ( curItem )
				{
				nextItem = item;
				idx = item->itemIndex;
				break;
				}
			}
		// remember the currently held item
		if ( item->itemID == curItemID
		&&	 item->itemEquipped )
			{
			curItem = item;
			}
		}
	
	// want to set the inventory command to:
	//	the ID of the next item
	//	the ID of the first item
	//	kPlyInvCmdEmptyLeft/Right (if it's a hand slot)
	//	eventually, for non-hand slots, need to send an unequip
	
	// if we found a next item then it's easy - just use its ID
	if ( nextItem )
		curItemID = nextItem->itemID;
	
	// we could fail to find a next item for two reasons
	// we were looking for the empty hand - use the first ID
	else
	if ( curItemID <= 0 )
		{
		if ( firstItem )
			{
			curItemID = firstItem->itemID;
			idx = firstItem->itemIndex;
			}
		}
	// or we are currently holding the last item in the list
	else
		{
		curItemID = slot == kItemSlotRightHand ? kPlyInvCmdEmptyRight
												: kPlyInvCmdEmptyLeft;
		idx = 0;
		}
	
	// save the command
	SendInvCmd( curItemID, idx + 1 );
}


/*
**	ClearInvCmd()
**
**	clear the inventory command if appropriate
*/
void
ClearInvCmd()
{
	int curInvCmd = gInvenCmd;
	
	switch ( curInvCmd )
		{
		case kPlyInvCmdNone:
			break;
		
		case kPlyInvCmdEmptyLeft:
			if ( 0 == gLeftItemID )
				curInvCmd = kPlyInvCmdNone;
			break;
		
		case kPlyInvCmdEmptyRight:
			if ( 0 == gRightItemID )
				curInvCmd = kPlyInvCmdNone;
			break;
		
		default:
			if ( curInvCmd == gLeftItemID
			||	 curInvCmd == gRightItemID )
				{
				curInvCmd = kPlyInvCmdNone;
				}
			break;
		}
	
	gInvenCmd = curInvCmd;
}


/*
**	UpdateInventoryWindow()
**
**	response to inventory info - redraw list
*/
void
UpdateInventoryWindow()
{
	gInvenList.AdjustRange();
	if ( InvenWindow * win = static_cast<InvenWindow *>( gInvenWindow ) )
		{
		win->Invalidate();
		win->UpdateInvenMenu();
		}
}


/*
**	UpdateItemPrefs()
**
**	convert the 'item' records in the prefs file to the current version
**	called when gPrefsData.pdItemPrefsVers != kItemPrefsVersion
*/
DTSError
UpdateItemPrefs()
{
	DTSError result = noErr;
	
	// update one version at a time
	while ( noErr == result
	&& 		gPrefsData.pdItemPrefsVers != kItemPrefsVersion )
		{
		switch ( gPrefsData.pdItemPrefsVers )
			{
			case 0:
			case 1:
				// we used to update to v2 here
				// but since items just changed totally
				// this is a good time to get rid of that code

//				result = -1;
//				break;
	//	it's OK to fall thru here, since we end up doing the same stuff
			
			case 2:
				// the upgrade from v2 to v3 is slightly too hard
				// so we'll just punt here too, forcing a full rebuild
				// of the item prefs; the poor players will just have to
				// reassign their item hotkeys. That's going to be more
				// reliable in the long run.
				result = -1;
				break;
			
			// add later revisions in here
			
			// any undefined transformations mean we just punt
			default:
				result = -1;
				break;
			}
		}
	
	// if there was an error, zap all the item prefs and let them be rebuilt at usual
	if ( result != noErr )
		{
		result = Delete1Type( &gClientPrefsFile, kTypeItemPrefs );

#if ITEM_PREFS_IN_PLIST
		Prefs::Set( pref_InventoryShortcut, nullptr );
		if ( sItemShortcutKeys )
			CFDictionaryRemoveAllValues( sItemShortcutKeys );
#endif  // ITEM_PREFS_IN_PLIST
		}
	
	// now we know the item prefs are up-to-date
	// (unless there was a disk error or something)
	if ( noErr == result )
		gPrefsData.pdItemPrefsVers = kItemPrefsVersion;
	
	return result;
}


#if TEST_EQUIP_ITEM
//
// =================== temporary test harness ==================
//

/*
**	HandleDoSendTextAEvt()
**
**	Accept a string of text from an AppleScript, then
**	attempt to wield the item with that name.
*/
static OSErr
HandleEquipItemAEvt( const AppleEvent * evt, AppleEvent * reply, SRefCon /* rc */ )
{
	// are we connected?
	if ( gPlayingGame )
		{
		char theText[ 256 ];
		DescType realType;
		SInt32 realSize;
		
		// extract text from evt
		OSStatus result = ::AEGetParamPtr( evt, keyDirectObject, typeText,
								&realType, &theText, sizeof theText, &realSize );
		if ( noErr == result )
			{
			// don't overflow the buffer
			if ( uint(realSize) >= sizeof theText )
				realSize = sizeof theText - 1;
			
			// C strings need trailing nulls
			theText[ realSize ] = '\0';
			
			// put that item into my hand
			const char * errorString = nullptr;
			result = EquipNamedItem( theText );
			switch ( result )
				{
				case noErr:
					{
					char buffer[ 256 ];
					// "You wield your %s."
					snprintf( buffer, sizeof buffer, _(TXTCL_WIELDING_ITEM), theText );
					ShowInfoText( buffer );
					}
					break;
				
				case -1:
					// "An inventory command is already in progress."
					errorString = _(TXTCL_INVENTORY_CMD_IN_PROGRESS);
					break;
				
				case -2:
					// "No item by that name."
					errorString = _(TXTCL_NO_ITEM_BY_THAT_NAME);
					break;
				
				default:
					// "Unknown error"
					errorString = _(TXTCL_UNKNOWN_ERROR);
				}
			
			// report failures
			if ( errorString )
				{
				AEPutParamPtr( reply, keyErrorString, typeUTF8Text,
					errorString, strlen( errorString ) );
				AEPutParamPtr( reply, keyErrorNumber, typeSInt32,
					&result, sizeof result );
				result = errAEEventNotHandled;
				}
			}
		return result;
		}
	
	// oh, the boundless futility of users.
	// "You are not connected to the game."
	const char * const errMsg = TXTCL_NOT_CONNECTED;
	AEPutParamPtr( reply, 'errs', typeUTF8Text, errMsg, strlen(errMsg) );
	return errAEEventNotHandled;
}


/*
**	InstallTextHandler()
**
**	tell OS about our event handler for the Do Script AppleEvent
*/
DTSError
InstallEquipItemHandler()
{
	AEEventHandlerUPP theUPP = NewAEEventHandlerUPP( HandleEquipItemAEvt );
	
	// install the handler
	return AddAppleEventHandler( kClientAppSign, 'Eqip', 0, theUPP );
}

#endif	// TEST_EQUIP_ITEM

