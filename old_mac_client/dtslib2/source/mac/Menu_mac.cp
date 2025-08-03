/*
**	Menu_mac.cp		dtslib2
**
**	Copyright 2023 Delta Tao Software, Inc.
** 
**	Licensed under the Apache License, Version 2.0 (the "License");
**	you may not use this file except in compliance with the License.
**	You may obtain a copy of the License at
** 
**		https://www.apache.org/licenses/LICENSE-2.0
** 
**	Unless required by applicable law or agreed to in writing, software
**	distributed under the License is distributed on an "AS IS" BASIS,
**	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
**	See the License for the specific language governing permissions and
**	imitations under the License.
*/

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "Local_dts.h"
#include "Menu_dts.h"
#include "Shell_mac.h"


/*
**	Entry Routines
**
**	DTSMenu
*/


/*
**	Definitions
*/
const MenuID	rDemoMenuID = 0x7FFF;

class DTSMenuPriv
{
public:
	bool			menuAutoKeys;
	static ushort	mHelpOffset;		// number of system items in Help menu
	
			// constructor/destructor
			DTSMenuPriv() :
				menuAutoKeys()
				{
				}
};


/*
**	Internal Routines
*/


/*
**	Global Variables
*/
DTSMenu *		gDTSMenu;			// global instance of the menu handler


/*
**	Internal Variables
*/
MenuItemIndex	DTSMenuPriv::mHelpOffset;


/*
**	DTSMenuPriv
*/
DTSDefineImplementFirmNoCopy(DTSMenuPriv)


/*
**	DTSMenu::DTSMenu()
*/
DTSMenu::DTSMenu()
{
#ifdef DEBUG_VERSION
	if ( gDTSMenu )
		VDebugStr( "Should not re-initialize the menu." );
#endif
	
	gDTSMenu = this;
}


/*
**	DTSMenu::~DTSMenu()
*/
DTSMenu::~DTSMenu()
{
#ifdef DEBUG_VERSION
	if ( gDTSMenu  &&  gDTSMenu != this )
		VDebugStr( "Global menu mismatch." );
#endif
	
	gDTSMenu = nullptr;
}


/*
**	DTSMenu::Init()
*/
DTSError
DTSMenu::Init( int menuBarID )
{
	DTSMenuPriv * p = priv.p;
	if ( not p )
		return -1;
	
	// load the menu bar
	Handle mBar = ::GetNewMBar( menuBarID );
	if ( not mBar )
		return -1;
	::SetMenuBar( mBar );
	
	// add the system-hosted Window menu if we can
	MenuRef stdWindowMenu = nullptr;
	OSStatus err = ::CreateStandardWindowMenu( kWindowMenuIncludeRotate, &stdWindowMenu );
	__Check_noErr( err );
	if ( noErr == err )
		::MacInsertMenu( stdWindowMenu, 0 );
	
	// add the PreRelease menu
	if ( bDebugMessages )
		{
#if 0
		MenuRef menu = ::NewMenu( rDemoMenuID, "\pPreRelease" );
		if ( not menu )
			return -1;
		
		const uchar * const prerelease =
				"\pDelta Tao Software Inc.;"
				"8032 Twin Oaks Ave.;"
				"Citrus Heights, CA 95610;"
				"408 730-9336";
		
		::AppendMenu( menu, prerelease );
#else
		MenuRef menu = nullptr;
		OSStatus err = ::CreateNewMenu( rDemoMenuID,
							kMenuAttrAutoDisable | kMenuAttrDoNotUseUserCommandKeys,
							&menu );
		__Check_noErr( err );
		if ( err )
			return err;
		
		__Verify_noErr( ::SetMenuTitleWithCFString( menu, CFSTR("PreRelease") ) );
		
# define AppendItem( str )	\
			::AppendMenuItemTextWithCFString( menu, CFSTR(str), kMenuItemAttrDisabled, 0, nullptr )
		
		AppendItem( "Delta Tao Software Inc." );
		AppendItem( "8032 Twin Oaks Ave." );
		AppendItem( "Citrus Heights, CA 95610" );
		AppendItem( "408 730-9336" );
		
# undef AppendItem
#endif  // 0
		
		::InsertMenu( menu, 0 );
		}
	
	::DrawMenuBar();
	
	return noErr;
}


/*
**	DTSMenu::DoCmd()
*/
bool
DTSMenu::DoCmd( long /* menuItem */ )
{
	return false;
}


/*
**	GetMenuHandleFromItem()
**
**	Extension of GetMenuHandle that knows about the Help menu.
**	The first time you call this with a menuItem from the
**	Help menu, it sets up a class variable (mHelpOffset) to be
**	the number of system-supplied items in the menu.
**	In Carbon, it also creates the Help menu if it doesn't already
**	exist (and if the MacHelp machinery is present.)
**
**	If the supplied menuItem does in fact refer to an item in the help
**	menu, then the value of the offset is added to the passed-in
**	parameter: the net effect is to skip over any items that had
**	been added by the OS, thereby making them "invisible" to clients
**	of DTSLib. In other words, DTSLib apps can number their help items
**	starting from 1, without needing to know what (or how many)
**	help items the OS has stuck in there first.
**
**	This scheme covers the mapping from command codes to menu items.
**	The reverse mapping (from menu items to command codes) has to be
**	handled at the point where those codes are created, e.g. after
**	calling MenuSelect() or MenuKey(). See DTSApp::KeyStroke() and
**	DoMouseDown(), in Shell_mac.cp, for that code. Briefly: those
**	functions take advantage of DTSMenu::GetHelpMenuOffset(), which
**	is just a simple accessor for the mHelpOffset class variable.
*/
static MenuRef
GetMenuHandleFromItem( int& menuItem )
{
	MenuRef menu = nullptr;
	OSStatus err = noErr;
	MenuID menuID = HiWord( menuItem );
	
	// are we dealing with the Help menu?
	if ( kHMHelpMenuID == menuID )
		{
		err = ::HMGetHelpMenu( &menu, &DTSMenuPriv::mHelpOffset );
		if ( noErr == err )
			{
			// HMGetHelpMenu() thoughtfully tells us the index of the
			// first custom item, so we just need to subtract 1 to
			// get a value commensurate with the OS9 measurement.
			DTSMenuPriv::mHelpOffset -= 1;
			
			menuItem += DTSMenuPriv::mHelpOffset;
			}
		}
	else
		{
		// any -other- menu but help: we just go right to the toolbox
		// for our answer
		menu = ::GetMenuHandle( menuID );
		}
	
	if ( err != noErr )
		menu = nullptr;
	
	return menu;
}


/*
**	DTSMenu::SetFlags()
*/
void
DTSMenu::SetFlags( int menuItem, uint flags )
{
	MenuRef menu = GetMenuHandleFromItem( menuItem );
	if ( not menu )
		return;
	
	MenuItemIndex theItem = LoWord( menuItem );
	
	if ( flags & kMenuDisabled )
		::DisableMenuItem( menu, theItem );
	else
		::EnableMenuItem( menu, theItem );
	
	// don't try to check item #0!
	if ( theItem != 0 )
		{
		bool bChecked = (flags & kMenuChecked ) != 0;
		::CheckMenuItem( menu, theItem, bChecked );
		}
	
	if ( 0 == theItem )
		::InvalMenuBar();
}


/*
**	DTSMenu::CheckRange()
*/
void
DTSMenu::CheckRange( int minItem, int maxItem, int checkItem )
{
	if ( MenuRef menu = GetMenuHandleFromItem( minItem ) )
		{
		for ( ;  minItem <= maxItem;  ++minItem )
			::CheckMenuItem( menu, static_cast<MenuItemIndex>( minItem ), (minItem == checkItem) );
		}
}


/*
**	DTSMenu::SetText()
**	if 'text' is NULL or "-", make it a separator
*/
void
DTSMenu::SetText( int menuItem, const char * text )
{
	// decompose menuItem into MenuRef + MenuItemIndex components
	MenuRef menu = GetMenuHandleFromItem( menuItem );
	if ( not menu )
		return;
	MenuItemIndex itm = LoWord( menuItem );
	
	bool bIsSeparator = (not text) || ('-' == text[0]);
	
	// extend the menu with blank items if 'itm' points beyond the current end
	for ( MenuItemIndex count = ::CountMenuItems( menu );  count < itm;  ++count )
		{
//		::AppendMenu( menu, "\p " );
		__Verify_noErr( ::AppendMenuItemTextWithCFString( menu,
							CFSTR(" "), kNilOptions, 0, nullptr ) );
		}
	
	// separators need special treatment
	if ( not bIsSeparator )
		{
		CFStringRef cs = CreateCFString( text );
		__Check( cs );
		if ( cs )
			{
			__Verify_noErr( ::SetMenuItemTextWithCFString( menu, itm, cs ) );
			CFRelease( cs );
			}
		}
	else
		{
		__Verify_noErr( ::ChangeMenuItemAttributes( menu, itm, kMenuItemAttrSeparator, 0 ) );
		}
}


/*
**	DTSMenu::GetText()
*/
void
DTSMenu::GetText( int menuItem, char * text, size_t textlen /*=sizeof(Str255)*/ ) const
{
	if ( MenuRef menu = GetMenuHandleFromItem( menuItem ) )
		{
		CFStringRef cs( nullptr );
		OSStatus err = CopyMenuItemTextAsCFString( menu, LoWord(menuItem), &cs );
		__Check_noErr( err );
		if ( noErr == err && cs )
			CFStringGetCString( cs, text, textlen, kCFStringEncodingMacRoman );
		
		if ( cs )
			CFRelease( cs );
		}
}


/*
**	DTSMenu::Delete()
*/
void
DTSMenu::Delete( int menuItem )
{
	if ( MenuRef menu = GetMenuHandleFromItem( menuItem ) )
		::DeleteMenuItem( menu, LoWord(menuItem) );
}


/*
**	DTSMenu::InsertMenu()
**
**  For inserting hierarchical menus: use beforeID = kInsertHierarchicalMenu
*/
void
DTSMenu::AddMenu( int menuID, int beforeID )
{
	if ( MenuRef menu = ::GetMenu( static_cast<MenuID>( menuID ) ) )
		::InsertMenu( menu, static_cast<MenuItemIndex>( beforeID ) );
}


/*
**	DTSMenu::GetAutoRepeat()
*/
bool
DTSMenu::GetAutoRepeat() const
{
	DTSMenuPriv * p = priv.p;
	return p ? p->menuAutoKeys : false;
}


/*
**	DTSMenu::GetHelpMenuOffset()
**
**	returns count of system-provided items in help menu, if we know it.
**	See the comment for GetMenuHandleFromItem(), above.
**	This has to be exposed thru DTSMenu because class DTSMenuPriv is
**	invisible to client code. (That's also why it can't be defined
**	in the declaration of class DTSMenu.)
*/
ushort
DTSMenu::GetHelpMenuOffset() const
{
	return DTSMenuPriv::mHelpOffset;
}


