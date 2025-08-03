/*
 *  Dialog_mach.cp
 *
 *  Created on 10/29/07.
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

#include "CarbonEvent_mac.h"
#include "Dialog_mach.h"


// do we use TransitionWindow() to fade non-sheet dialogs in and out of view?
// (kinda pretty, but awfully slow...)
#define USE_WINDOW_FADE_EFFECTS		0


/*
**	HIDialog::TearDown()
**
**	the dialog has been dismissed, so release all its support machinery
*/
OSStatus
HIDialog::TearDown()
{
	// lose the CarbonEvent stuff
	OSStatus result = RemoveHandler();
	__Check_noErr( result );
	
	// and close the window
	if ( mWindow )
		{
		DisposeWindow( mWindow );
		mWindow = nullptr;
		}
	
	SetThemeCursor( kThemeArrowCursor );
	return result;
}


/*
**	MyGetWindowBounds()
**
**	wrapper for HIWindowGetBounds()
*/
static inline OSStatus
MyGetWindowBounds( WindowRef w, HIRect& oRect )
{
	return HIWindowGetBounds( w, kWindowStructureRgn, kHICoordSpace72DPIGlobal, &oRect );
}


/*
**	MySetWindowBounds
**
**	wrapper for HIWindowSetBounds()
*/
static inline OSStatus
MySetWindowBounds( WindowRef w, const HIRect& b )
{
	return HIWindowSetBounds( w, kWindowStructureRgn, kHICoordSpace72DPIGlobal, &b );
}


/*
**	MyConstrainWindow()
**
**	wrapper for HIWindowConstrain()
*/
static inline OSStatus
MyConstrainWindow( WindowRef w )
{
	return HIWindowConstrain( w, kWindowStructureRgn,
								kWindowConstrainStandardOptions,
								kHICoordSpace72DPIGlobal, nullptr, nullptr, nullptr );
}


/*
**	ApplyPositionPref()
**
**	called for non-sheet windows, just before showing them for the first time.
**	If the subclass returns a valid pref key, we try to get a saved bounds rect from it,
**	and resize & reposition the window accordingly
*/
static void
ApplyPositionPref( WindowRef mWindow, CFStringRef prefKey )
{
	// if the window can't resize, don't bother
	// ... but then again, it _can_ be moved
	// ... but on the third hand, if the nib size changes due to a redesign,
	// we should respect that new size, not the saved one from the prefs
//	if ( not HIWindowTestAttribute( mWindow, kHIWindowBitResizable ) )
//		return;
	
	// got a valid prefs key?
	if ( CFStringGetLength( prefKey ) > 1 )
		{
		// try to get the saved rect
		HIRect savedR = Prefs::GetRect( prefKey, CGRectNull );
		if ( not CGRectIsNull( savedR ) )
			{
			// for non-resizable windows, use the current height&width
			if ( not HIWindowTestAttribute( mWindow, kHIWindowBitResizable ) )
				{
				HIRect curRect;
				__Verify_noErr( MyGetWindowBounds( mWindow, curRect ) );
				savedR.size = curRect.size;
				}
			
			// apply it!
			__Verify_noErr( MySetWindowBounds( mWindow, savedR ) );
			
			// in case it was nonsensical, or screen geometry has changed...
			__Verify_noErr( MyConstrainWindow( mWindow ) );
			}
		}
}


/*
**	RecordPositionPref()
**
**	Counterpart of the above: just before hiding a non-sheet dialog, we'll
**	save its current bounds for posterity (if the subclass provides a valid prefs key)
*/
static void
RecordPositionPref( WindowRef mWindow, CFStringRef prefKey )
{
	// if the window can't resize, don't bother
	// ... but then again, it _can_ be moved
//	if ( not HIWindowTestAttribute( mWindow, kHIWindowBitResizable ) )
//		return;
	
	// pref key OK?
	if ( CFStringGetLength( prefKey ) > 1 )
		{
		// get current bounds, and squirrel them away
		HIRect r;
		if ( noErr == MyGetWindowBounds( mWindow, r ) )
			{
			Prefs::Set( prefKey, r );
			}
		}
}


/*
**	HIDialog::GetNibFileName()
**
**	The name of the nib file containing the definition of this dialog's window,
**	minus the ".nib" or ".xib" extension.  Subclasses MUST implement this.
*/
CFStringRef
HIDialog::GetNibFileName() const
{
#ifdef DEBUG_VERSION
	VDebugStr( "No nibfile name provided!" );
#endif
	return nullptr;
}


/*
**	HIDialog::GetNibWindowName()
**
**	The name of the window definition within the nib file.
**	It seems to be quite common for the window and file names to be the same;
**	this function handles that commonality.  If they differ, the subclass must override.
*/
CFStringRef
HIDialog::GetNibWindowName() const
{
	return GetNibFileName();
}


/*
**	IsWindowClassValid()
**
**	check that this dialog is operating in an appropriate window class
*/
static bool
IsWindowClassValid( bool isSheet, WindowClass wc )
{
	// sheet dialogs must use kSheetWindowClass
	if ( isSheet )
		return kSheetWindowClass == wc;
	
	// non-sheets have a bit more leeway
	switch ( wc )
		{
//		case kAlertWindowClass:		// should upgrade to movable variant
		case kMovableAlertWindowClass:
//		case kModalWindowClass:		// ditto
		case kMovableModalWindowClass:
		case kPlainWindowClass:
		case kAltPlainWindowClass:
		case kSimpleWindowClass:
			return true;		// all those are OK
		}
	
	return false;	// all others no good
}


/*
**	HIDialog::Run()
**
**	present the dialog, and drop into a window-modal event loop until it's dismissed.
**	optionally, though, display it as a sheet instead -- in which case, no event loop.
*/
OSStatus
HIDialog::Run( WindowRef parent /* = nullptr */ )
{
	// fetch the dialog contents from the nib file
	IBNibRef nib;
	OSStatus err = CreateNibReference( GetNibFileName(), &nib );
	__Check_noErr( err );
	if ( noErr == err )
		{
		err = CreateWindowFromNib( nib, GetNibWindowName(), &mWindow );
		__Check_noErr( err );
		}
	if ( nib )
		DisposeNibReference( nib );
	
	// validate window class (sheet or not)
	if ( noErr == err )
		{
		WindowClass wClass = 0;
		__Verify_noErr( GetWindowClass( mWindow, &wClass ) );
		if ( not IsWindowClassValid( parent != nullptr, wClass ) )
			{
#ifdef DEBUG_VERSION
			VDebugStr( "Wrong window class for dialog!" );
#endif
			err = paramErr;
			}
		}
	
	// attach the carbon events
	if ( noErr == err )
		err = InitTarget( GetWindowEventTarget( mWindow ) );
	if ( noErr == err )
		{
		const EventTypeSpec evt = { kEventClassWindow, kEventWindowCursorChange };
		err = AddEventType( evt );
		}
	
	if ( noErr == err )
		{
		// call subclass, to pre-populate the fields and controls and stuff
		Init();
		
		// are we app-modal, or a sheet?
		if ( parent )
			{
			mParent = parent;
			err = ShowSheetWindow( mWindow, parent );
			__Check_noErr( err );
			}
		else
			{
			// check if there's a saved bounds pref, and try to use it
			if ( CFStringRef prefKey = GetPositionPrefKey() )
				ApplyPositionPref( mWindow, prefKey );
			
#if USE_WINDOW_FADE_EFFECTS
			__Verify_noErr( TransitionWindow( mWindow, kWindowFadeTransitionEffect,
							 kWindowShowTransitionAction, nullptr ) );
#else
			ShowWindow( mWindow );
#endif
			}
		
		// put the user focus onto the right starting point
		if ( noErr == err && WantAutoFocusAdvance() )
			{
			(void) HIViewAdvanceFocus( HIViewGetRoot( mWindow ), 0 );
//			__Check_noErr( err );
			}
		
		// let the (non-sheet) dialog run until it dismisses itself
		if ( noErr == err && not mParent )
			err = RunAppModalLoopForWindow( mWindow );
		}
	
	return err;
}


/*
**	HIDialog::ProcessCommand()
**
**	react to HICommands for this dialog.
**	Presently, we only do OK and Cancel. Subclasses can of course handle whatever they wish.
*/
OSStatus
HIDialog::ProcessCommand( const HICommandExtended& cmd, CarbonEvent& evt )
{
	// first attempt to dispatch this command to subclasses using a simpler interface.
	// If they implement DoCommand() and return true, we're done.
	// If they want/need the full generality of ProcessCommand(), they can just return false
	// from DoCommand() -- or not implement it at all.
	
	if ( DoCommand( cmd ) )
		return noErr;
	
	OSStatus result = eventNotHandledErr;
	switch ( cmd.commandID )
		{
		// either an OK or a Cancel command will dismiss the dialog.
		// subclasses' Exit() hooks can inspect the mReason field to learn which one it was.
		// However a subclass can override the Validate() method; if that returns false then
		// the OK command will _not_ dismiss.
		case kHICommandOK:
			if ( Validate() )
				Dismiss( kHICommandOK );
			result = noErr;
			break;
		
		case kHICommandCancel:
			Dismiss( kHICommandCancel );
			result = noErr;
			break;
		}
	
	if ( eventNotHandledErr == result )
		result = SUPER::ProcessCommand( cmd, evt );
	
	return result;
}


/*
**	HIDialog::Dismiss()
**
**	we've been OK'd or Canceled: terminate the dialog event loop
*/
void
HIDialog::Dismiss( UInt32 commandID )
{
	// record the dismissing command, for subclass use
	mReason = commandID;
	
	// tell subclass we are going away
	Exit();
	
	if ( mWindow )
		{
		// terminate the modal event loop, for non-sheets
		if ( not mParent )
			QuitAppModalLoopForWindow( mWindow );
		
		// if we were a sheet, roll ourselves back up. Otherwise, just hide.
		if ( mParent )
			{
			__Verify_noErr( HideSheetWindow( mWindow ) );
			}
		else
			{
			// maybe record window-position pref
			if ( CFStringRef prefKey = GetPositionPrefKey() )
				RecordPositionPref( mWindow, prefKey );
			
#if USE_WINDOW_FADE_EFFECTS
			__Verify_noErr( TransitionWindow( mWindow, kWindowFadeTransitionEffect,
							kWindowHideTransitionAction, nullptr ) );
#else
			HideWindow( mWindow );
#endif
			}
		}
	
	// if we were a sheet, delete ourselves
	if ( mParent )
		delete this;
}


/*
**	HIDialog::GetItem()
**
**	bottleneck function for retrieving a specific "dialog item"
*/
HIViewRef
HIDialog::GetItem( const HIViewID& itemID ) const
{
	HIViewRef vu = nullptr;
	if ( mWindow )
		__Verify_noErr( HIViewFindByID( HIViewGetRoot( mWindow ), itemID, &vu ) );
	
	return vu;
}


/*
**	HIDialog::GetValue()
**
**	returns the value of an item (control) in this dialog.
**	NB: edit-text fields don't have values, they have content!
*/
int
HIDialog::GetValue( const HIViewID& itemID ) const
{
	int result = 0;
	if ( HIViewRef vu = GetItem( itemID ) )
		result = HIViewGetValue( vu );
	
	return result;
}


/*
**	HIDialog::SetValue()
**
**	change the value of a control item
*/
OSStatus
HIDialog::SetValue( const HIViewID& itemID, int value ) const
{
	OSStatus err = paramErr;
	if ( HIViewRef vu = GetItem( itemID ) )
		{
		err = HIViewSetValue( vu, value );
		__Check_noErr( err );
		}
	return err;
}


/*
**	HIDialog::CopyText()
**
**	retrieve the text of a dialog item (e.g. static or edit text fields)
**	NB if the item doesn't have real text content, as such, this will fetch its
**	title/caption instead, which is usually not what you want.
*/
CFStringRef
HIDialog::CopyText( const HIViewID& itemID ) const
{
	CFStringRef result = nullptr;
	if ( HIViewRef vu = GetItem( itemID ) )
		result = HIViewCopyText( vu );
	return result;
}


/*
**	HIDialog::GetText()
**
**	retrieve text into a buffer of chars, using a specified text encoding.
**	The buffer is nul-terminated.
*/
bool
HIDialog::GetText( const HIViewID& itemID, char * buff, size_t bufsz,
					 CFStringEncoding enc /* = kCFStringEncodingMacRoman */ ) const
{
	__Check( buff );
	if ( not buff )
		return false;
	
	__Check( bufsz > 0 );
	
	bool result = false;
	if ( CFStringRef cs = CopyText( itemID ) )
		{
		// This next call can fail if: (1) buffer is too small, or
		// (2) characters in 'cs' are incompatible with given encoding.
		// If you have any reason to worry about either possibility, you should be using
		// CopyText() directly.
		
		// As a last-ditch safety, though, we do some crude failure tests.
		
		result = CFStringGetCString( cs, buff, bufsz, enc );
		if ( not result )
			{
			const uchar kLossByte = 0xC0;	// MacRoman inverted question-mark
			
			// get what we can, replacing unencodable characters with the loss byte
			CFIndex len = CFStringGetLength( cs );
			CFIndex usedLen;
			CFIndex numGot = CFStringGetBytes( cs, CFRangeMake( 0, len ), enc, kLossByte, false,
												(uchar *) buff, bufsz, &usedLen );
			buff[ usedLen ] = '\0';
			
			// tell 'em what we did.
			// We aren't necessarily being called by Exit(), so let's update the contents
			// of the edit field.
			if ( CFStringRef clean = CFStringCreateWithBytes( kCFAllocatorDefault,
										(const uchar *) buff, strlen( buff ),
										enc, false ) )
				{
				SetText( itemID, clean );
				CFRelease( clean );
				}
			
			// hilite the bad field, so they'll have some context while pondering the
			// error alert we're about to show.  (But the field might be on a currently-hidden
			// pane of a multi-pane dialog, or otherwise out of sight, in which case, don't bother)
			if ( HIViewRef vu = GetItem( itemID ) )
				{
				if ( HIViewIsVisible( vu ) )
					SelectText( itemID, 0, 0x3FFF );
				}
			
			// did we fetch every character (possibly converting some)?
			// -- this could fail for stuff like unicode combining characters, couldn't it?
			if ( numGot == len )
				{
				// the length was OK, so there must have been bad characters
				GenericError( "Bad character in field\n"
						"Warning: a field contained one or more invalid (non-MacRoman) "
						"characters, which have been replaced by a \xD4\xC0\xD5 mark." );
				}
			else
				{
				// otherwise the length must have been too short.
				// Not going to worry about that; we'll just silently truncate, just like
				// the classic DTSLib dialogs have always done.
				}
			}
		CFRelease( cs );
		}
	else
		{
		// uh-oh, no text at all!?!
		*buff = '\0';
		}
	
	return result;
}


/*
**	HIDialog::SetText()
**
**	install new text into a dialog item
*/
OSStatus
HIDialog::SetText( const HIViewID& itemID, CFStringRef text ) const
{
	OSStatus result = paramErr;
	if ( HIViewRef vu = GetItem( itemID ) )
		result = HIViewSetText( vu, text );
	return result;
}


/*
**	HIDialog::SetText() -- C string version
**
**	Same as above, but takes a traditional MacRoman string instead of a CFStringRef.
**	Practically all of the CL data structures are in that form, so this is quite handy.
*/
OSStatus
HIDialog::SetText( const HIViewID& itemID, const char * text ) const
{
	OSStatus result = paramErr;
	if ( HIViewRef vu = GetItem( itemID ) )
		{
		CFStringRef cs = CreateCFString( text );
		__Check( cs );
		if ( cs )
			{
			result = HIViewSetText( vu, cs );
			CFRelease( cs );
			}
		else
			result = memFullErr;	// ???
		}
	return result;
}


/*
**	HIDialog::SelectText()
**
**	select an edit-text field, or a range thereof
*/
OSStatus
HIDialog::SelectText( const HIViewID& itemID, short begin /* =0 */, short end /* =7FFF */ ) const
{
	OSStatus result = paramErr;
	if ( HIViewRef vu = GetItem( itemID ) )
		{
		result = HIViewSetFocus( vu, kControlEditTextPart, kNilOptions );
												// or kHIViewFocusTraditionally?
		__Check_noErr( result );
		
		// my plan was that SelectText(f) would merely move the focus to the field without
		// modifying the insertion/selection position, whereas a full-fledged
		// SelectText(f,b,e) would accomplish both.
		// but I'm not finding a lot of use cases for the former, which means you end up
		// having to write the 3-arg version each time... even though the idea of optional
		// args is to _simplify_ life. Feh.
		if ( noErr == result && (begin != 0 || end != 0x7FFF) )
			{
			ControlEditTextSelectionRec sel = { begin, end };
			result = SetControlData( vu, kControlEditTextPart, kControlEditTextSelectionTag,
							sizeof sel, &sel );
			__Check_noErr( result );
			}
		}
	
	return result;
}


/*
**	HIDialog::SetNumber()
**
**	put an integer numeric value into a dialog item, as text -- NOT the same as SetValue()!
*/
OSStatus
HIDialog::SetNumber( const HIViewID& itemID, int value ) const	// for text fields
{
	OSStatus result = paramErr;
	if ( HIViewRef vu = GetItem( itemID ) )
		{
		CFStringRef cs = CFStringCreateFormatted( CFSTR("%d"), value );
		__Check( cs );
		if ( cs )
			{
			result = HIViewSetText( vu, cs );
			CFRelease( cs );
			}
		else
			result = memFullErr;
		}
	return result;
}


/*
**	HIDialog::GetNumber()
**
**	Attempt to retrieve the numeric value of the text of a dialog item.
**	This is only for simple dialog input fields, so we take no heroic measures to interpret
**	poorly-formed (non-numeric) text.  Returns 0 if the text can't be converted to a number.
**	(Compare to DTSDlgView, which returns 0x80000000 if it couldn't parse anything).
**
**	That said, if you supply the strictlyValid argument, we'll set that to true if (and only if)
**	the text of the input field was a well-formed integer.
*/
int
HIDialog::GetNumber( const HIViewID& itemID, bool * strictlyValid /* =nullptr */ ) const
{
	// guilty until proved innocent
	if ( strictlyValid )
		*strictlyValid = false;
	
	int result = 0;
	if ( CFStringRef cs = CopyText( itemID ) )
		{
		result = CFStringGetIntValue( cs );
		
		// do we need to attempt strict(er) validation?
		if ( strictlyValid )
			{
			// assume the worst
			*strictlyValid = false;
			
#if 0
			char buff[ 256 ];
			if ( CFStringGetCString( cs, buff, sizeof buff, kCFStringEncodingUTF8 ) )
				{
				char * endptr;
				long x = strtol( buff, &endptr, 10 );
				if ( endptr && '\0' == *endptr && x == result )
					*strictlyValid = true;
				}
#else
			if ( CFNumberFormatterRef fmt = CFNumberFormatterCreate( kCFAllocatorDefault,
													nullptr, kCFNumberFormatterNoStyle ) )
				{
				CFIndex slen = CFStringGetLength( cs );
				CFRange range = CFRangeMake( 0, slen );
				int x;
				//
				// docs say you can pass NULL as the last argument here, if you don't actually
				// need the parsed value: but it always crashes when I try that.
				//
				if ( CFNumberFormatterGetValueFromString( fmt, cs, &range, kCFNumberIntType, &x ) )
					{
					if ( 0 == range.location
					&&   slen == range.length )
						{
						__Check( x == result );
						*strictlyValid = true;
						}
					}
				CFRelease( fmt );
				}
#endif  // 0
			}
		CFRelease( cs );
		}
	return result;
}


/*
**	HIDialog::SetFloat()
**
**	just like the DTSDlgView version
**	probably should have a way to modify the conversion format... 
*/
OSStatus
HIDialog::SetFloat( const HIViewID& itemID, double value ) const
{
	OSStatus err = coreFoundationUnknownErr;
	
	char buff[ 32 ];
	int len = snprintf( buff, sizeof buff, "%.8lf", value );
	
	// remove all trailing zeros
	char * ptr = buff + len - 1;
	while ( '0' == *ptr )
		*ptr-- = '\0';
	
	if ( CFStringRef cs = CreateCFString( buff, kCFStringEncodingASCII ) )
		{
		err = SetText( itemID, cs );
		CFRelease( cs );
		}
	
	return err;
}


/*
**	BaseDlg::GetFloat()
**
**	just like the DTSDlgView version
*/
double
HIDialog::GetFloat( const HIViewID& itemID ) const
{
	double result = 0;
	
	char buff[ 32 ];
	if ( GetText( itemID, buff, sizeof buff, kCFStringEncodingASCII ) )
		sscanf( buff, "%lf", &result );
	
	return result;
}


#if MAC_OS_X_VERSION_MAX_ALLOWED > MAC_OS_X_VERSION_10_4
/*
**	HIDialog::SetIcon()
**
**	updates an dialog icon-view item to refer to the desired IconRef
**	(also works on things like bevel buttons, image wells, etc.)
**	Requires OS 10.5 or later.
*/
OSStatus
HIDialog::SetIcon( const HIViewID& itemID, OSType iconID ) const
{
# if MAC_OS_X_VERSION_MIN_REQUIRED < MAC_OS_X_VERSION_10_5
	if ( not IsCFMResolved( HIViewSetImageContent ) )
		return paramErr;
# endif
	
	OSStatus err = paramErr;
	if ( HIViewRef vu = GetItem( itemID ) )
		{
		HIViewContentInfo btnInfo;
		btnInfo.contentType = kHIViewContentIconTypeAndCreator;
		btnInfo.u.iconTypeAndCreator.creator =
			gDTSApp ? gDTSApp->GetSignature() : '???\?';
		btnInfo.u.iconTypeAndCreator.type = iconID;
		err = HIViewSetImageContent( vu, kHIViewEntireView, &btnInfo );
		__Check_noErr( err );
		}
	
	return err;
}
#endif  // 10.5+


/*
**	HIDialog::CountPopupMenuItems()
**
**	This returns a 1-based count of the number of menu items, within a given
**	popup menu control.
**	Ordinarily this should always be the same as HIViewGetMaximum(), but this way
**	seems more bullet-proof.
*/
int
HIDialog::CountPopupMenuItems( const HIViewID& itemID ) const
{
	if ( HIViewRef control = GetItem( itemID ) )
		if ( MenuRef menu = GetControlPopupMenuHandle( control ) )
			return CountMenuItems( menu );
	
	return 0;
}


/*
**	HIDialog::SetPopupText()
**
**	Set the text of a specific item within a popup menu control. 'menuItem' is 1-based.
**	If 'text' is NULL, make that item a separator.
**
**	This code slavishly imitates the semantics of DTSLib, whereby if 'menuItem' is greater
**	than the number of items already present in the menu, then we grow the menu to the
**	required size by appending the requisite number of new (empty) items, in front of the
**	one being explicitly set.
*/
void
HIDialog::SetPopupText( const HIViewID& itemID, int menuItem, CFStringRef text ) const
{
	// get the control handle
	if ( HIViewRef control = GetItem( itemID ) )
		{
		// extract the MenuRef from the control
		// bail if there is none
		if ( MenuRef menu = GetControlPopupMenuHandle( control ) )
			{
			// insert any extra blank lines
			int count = CountMenuItems( menu );
			for ( ; count < menuItem; ++count )
				__Verify_noErr( AppendMenuItemTextWithCFString( menu,
					CFSTR(" "), kNilOptions, 0, nullptr ) );
			
			HIViewSetMaximum( control, count );
			
			// a null CFString is a separator line
			if ( not text
			||   kCFCompareEqualTo == CFStringCompare( text, CFSTR("-"), kNilOptions ) )
				{
				// make this a separator
				__Verify_noErr( ChangeMenuItemAttributes( menu, menuItem,
						kMenuItemAttrDisabled | kMenuItemAttrSeparator, 0 ) );
				}
			else
				{
				// set the menu text
				__Verify_noErr( SetMenuItemTextWithCFString( menu, menuItem, text ) );
				}
			}
		}
}


/*
**	HIDialog::CopyPopupText()
**
**	Retrieve the text of a popup menu item. NB: 'menuItem' is 1-based.
**	Returns NULL if the item doesn't exist, or isn't a popup menu, or doesn't have that item.
*/
CFStringRef
HIDialog::CopyPopupText( const HIViewID& itemID, int menuItem ) const
{
	CFStringRef cs = nullptr;
	
	if ( HIViewRef control = GetItem( itemID ) )
		if ( MenuRef menu = GetControlPopupMenuHandle( control ) )
			{
			__Verify_noErr( CopyMenuItemTextAsCFString( menu, menuItem, &cs ) );
			}
	
	return cs;
}


/*
**	HIDialog::SetVisible()
**
**	show or hide a dialog item
*/
OSStatus
HIDialog::SetVisible( const HIViewID& itemID, bool vis ) const
{
	OSStatus result = paramErr;
	if ( HIViewRef vu = GetItem( itemID ) )
		{
		result = HIViewSetVisible( vu, vis );
		__Check_noErr( result );
		}
	return result;
}


/*
**	HIDialog::IsItemVisible()
**
**	a getter to correspond with the above setter
*/
bool
HIDialog::IsItemVisible( const HIViewID& itemID ) const
{
	if ( HIViewRef vu = GetItem( itemID ) )
		return HIViewIsVisible( vu );
	return false;
}


/*
**	HIDialog::AbleItem()
**
**	Something of a linguistic innovation! This either EN-ables or DIS-ables a dialog item.
*/
OSStatus
HIDialog::AbleItem( const HIViewID& itemID, bool enable ) const
{
	OSStatus result = paramErr;
	if ( HIViewRef vu = GetItem( itemID ) )
		{
		result = HIViewSetEnabled( vu, enable );
		__Check_noErr( result );
		}
	return result;
}


/*
**	HIDialog::IsItemEnabled()
**
**	getter for the above
*/
bool
HIDialog::IsItemEnabled( const HIViewID& itemID ) const
{
	if ( HIViewRef vu = GetItem( itemID ) )
		return HIViewIsEnabled( vu, nullptr );
	return false;
}



/*
**	HIDialog::SetTitle()
**
**	modify the contents of a dialog window's title bar
*/
OSStatus
HIDialog::SetTitle( CFStringRef wTitle ) const	// not for sheets -- duh
{
	OSStatus result = paramErr;
	if ( mWindow && not mParent )
		{
		result = SetWindowTitleWithCFString( mWindow, wTitle );
		__Verify_noErr( result );
		}
	return result;
}


/*
**	HIDialog::SetTitle()
**
**	modify the contents of a dialog window's title bar
**	MacRoman, const char * flavor
*/
OSStatus
HIDialog::SetTitle( const char * wTitle, CFStringEncoding encoding /* = macroman */  ) const
{
	OSStatus result = coreFoundationUnknownErr;
	if ( CFStringRef cs = CreateCFString( wTitle, encoding ) )
		{
		result = SetTitle( cs );
		CFRelease( cs );
		}
	
	return result;
}


/*
**	HIDialog::DoViewEvent()		[static]
**
**	react to CarbonEvents aimed at our subcontrols
**	currently we only care about TextField events: we redispatch those to HandleTextFieldEvent()
*/
OSStatus
HIDialog::DoViewEvent( EventHandlerCallRef call, EventRef event, void * ud )
{
	OSStatus result = eventNotHandledErr;
	try
		{
		const CarbonEvent evt( event, call );
		
		HIViewRef vu = nullptr;
		__Verify_noErr( evt.GetParameter( kEventParamDirectObject,
							typeControlRef, sizeof vu, &vu ) );
		if ( vu )
			{
			HIDialog * dlg = static_cast<HIDialog *>( ud );
			result = dlg->HandleTextFieldEvent( vu, evt );
			}
		}
	catch ( ... ) {}
	
	return result;
}


/*
**	HIDialog::SetFieldNotifications()
**
**	although we could, I suppose, automatically enable TextField events for every
**	edit-text control in every dialog, that seems unnecessary and wasteful. Instead,
**	therefore, subclasses are required to explicitly subscribe to those events on an
**	item-by-item basis.
**
**	This routine does the necessary plumbing to activate such subscriptions.
*/
OSStatus
HIDialog::SetFieldNotifications( const HIViewID& fieldID, TFNotification notifs )
{
	// for now, squawk if no notifications are requested.
	// Eventually it may be desirable to permit "unsubscription" requests too.
	enum { kTFNotifyAll = kTFNotifyAccept | kTFNotifyShouldChange | kTFNotifyDidChange };
	if ( not (notifs & kTFNotifyAll) )
		return paramErr;
	
	OSStatus result = noErr;
	if ( HIViewRef vu = GetItem( fieldID ) )
		{
		// we'll want any or all of the following 3 text-field events:
		EventTypeSpec whichEvents[3] = {
			{ kEventClassTextField, 0 },
			{ kEventClassTextField, 0 },
			{ kEventClassTextField, 0 }
			};
		UInt32 nEvents = 0;
		if ( notifs & kTFNotifyAccept )
			whichEvents[ nEvents++ ].eventKind = kEventTextAccepted;
		if ( notifs & kTFNotifyShouldChange )
			whichEvents[ nEvents++ ].eventKind = kEventTextShouldChangeInRange;
		if ( notifs & kTFNotifyDidChange )
			whichEvents[ nEvents++ ].eventKind = kEventTextDidChange;
		
		EventHandlerUPP upp = NewEventHandlerUPP( DoViewEvent );
		result = InstallControlEventHandler( vu, upp, nEvents, whichEvents, this, nullptr );
		}
	
	return result;
}


/*
**	HIDialog::HandleTextFieldEvent()
**
**	react to one of the TextField events occurring within a dialog text item.
**	We do the grunt work of decoding here, and then inform our subclass, via the 
**	Field{Did,Should}{Accept,Change}Text() virtual methods.
*/
OSStatus
HIDialog::HandleTextFieldEvent( HIViewRef vu, const CarbonEvent& evt )
{
	OSStatus result = eventNotHandledErr;
	if ( kEventClassTextField == evt.Class() )
		{
		switch ( evt.Kind() )
			{
				// notification that the user has accepted his/her changes to a text field
				// (e.g. by tabbing out of the field, or pressing return/enter, etc.)
			case kEventTextAccepted:
				if ( FieldDidAcceptText( vu ) )
					result = noErr;
				break;
				
				// see documentation of kEventTextShouldChangeInRange
			case kEventTextShouldChangeInRange:
				result = FieldShouldChangeText( vu, evt );
				break;
			
				// notification that the user has changed the contents of a text field
				// (but has not yet necessarily accepted those changes; more may be coming)
			case kEventTextDidChange:
				{
				// don't handle this event unless the unconfirmed-range param is _not_ present
				CFRange range;
				if ( noErr != evt.GetParameter( kEventParamUnconfirmedRange, typeCFRange,
									sizeof range, &range ) )
					{
					if ( FieldDidChangeText( vu ) )
						result = noErr;
					}
				}
				break;
			}
		}
	
	return result;
}


/*
**	HIDialog::HandleEvent()
**
**	handle cursor-change events for text fields
*/
OSStatus
HIDialog::HandleEvent( CarbonEvent& evt )
{
	// ignore any event that's not a cursor-change for our window,
	// including whenever we're not frontmost
	if ( kEventClassWindow == evt.Class()
	&&   kEventWindowCursorChange == evt.Kind()
	&&   IsWindowActive( mWindow )
	&&   ActiveNonFloatingWindow() == mWindow )	// paranoia
		{
		// not sure if we can count on getting the window-relative loc...
		HIPoint mLoc;
		OSStatus err = evt.GetParameter( kEventParamWindowMouseLocation,
							typeHIPoint, sizeof mLoc, &mLoc );
		if ( eventParameterNotFoundErr == err )
			{
			// ... so fall back to the global loc, which should always be there
			err = evt.GetParameter( kEventParamMouseLocation,
						typeHIPoint, sizeof mLoc, &mLoc );
			if ( noErr == err )
				{
				// convert from global to window, so the next paragraph gets consistent inputs
				HIPointConvert( &mLoc, kHICoordSpace72DPIGlobal, nullptr,
										kHICoordSpaceWindow, mWindow );
				}
			}
		if ( noErr == err )
			{
			// the point is now in window-structure-local coordinates
			HIViewRef contentVu = nullptr;
			err = HIViewFindByID( HIViewGetRoot( mWindow ), kHIViewWindowContentID, &contentVu );
			if ( noErr == err )
				{
				// finally make the coords window-content-local, and test what we hit
				HIPointConvert( &mLoc, kHICoordSpaceWindow, mWindow, kHICoordSpaceView, contentVu );
				DoCursorChange( mLoc, contentVu );
				}
			}
		}
	
	return HICommandResponder::HandleEvent( evt );
}


/*
**	IsTextView()
**
**	is this view a text-editing view, i.e., is it likely to want an I-beam cursor?
*/
static bool
IsTextView( HIViewRef vu )
{
	bool result = false;
	
	HIViewKind kind;
	if ( noErr == HIViewGetKind( vu, &kind )
	&&   kHIViewKindSignatureApple == kind.signature )
		{
		if ( kControlKindHITextView		 == kind.kind
		||   kControlKindEditText		 == kind.kind
		||   kControlKindEditUnicodeText == kind.kind
		||   kControlKindHISearchField	 == kind.kind
		||   kControlKindHIComboBox		 == kind.kind
		||   kControlKindClock			 == kind.kind )
			{
			result = true;
			}
		}
	
	return result;
}


/*
**	HIDialog::DoCursorChange()
**
**	some text-input controls unilaterally set the I-beam cursor when you click in them,
**	but nothing in the system makes provision for restoring the arrow cursor when you
**	leave them.  This function is called frequently, via kEventWindowCursorChanged,
**	to do that very thing.
**
**	Should probably be done via tracking areas, but then I'd have to scan every single
**	view in the window to find their types, and their bounding boxes.
*/ 
void
HIDialog::DoCursorChange( const HIPoint& loc, HIViewRef contentView ) const
{
	// assume we're not over a text field
	ThemeCursor newCursor = kThemeArrowCursor;
	
	// find the view under the cursor
	HIViewRef vu = nullptr;
	if ( noErr == HIViewGetSubviewHit( contentView, &loc, true, &vu ) )
		{
		if ( IsTextView( vu ) )
			{
			// it IS a text control: it needs an i-beam
			newCursor = kThemeIBeamCursor;
			}
		}
	
	// put up the appropriate cursor
	// (if it has changed)
	if ( newCursor != mLastCursor )
		{
		SetThemeCursor( newCursor );
		mLastCursor = newCursor;
		}
}


#if MAC_OS_X_VERSION_MAX_ALLOWED > MAC_OS_X_VERSION_10_4
/*
**	HIDialog::SetFieldLengthLimit()
**
**	Set a length limit on a text-input field
**	Requires OSX 10.5 or later at runtime.
*/
OSStatus
HIDialog::SetFieldLengthLimit( const HIViewID& fieldID, UInt32 maxLength )
{
# if MAC_OS_X_VERSION_MIN_REQUIRED < MAC_OS_X_VERSION_10_5
	if ( not IsCFMResolved( HIObjectAddDelegate ) )
		return paramErr;
# endif
	
	// HIViews are a subclass of HIObjectRef, but C++ doesn't know that.
	HIObjectRef vu = reinterpret_cast<HIObjectRef>( GetItem( fieldID ) );
	__Check( vu );
	if ( not vu )
		return paramErr;
	
	HIObjectRef limiter = nullptr;
	CarbonEvent initEvent( kEventClassHIObject, kEventHIObjectInitialize );
	initEvent.SetParameter( kEventParamTextLength, typeUInt32, sizeof maxLength, &maxLength );
	
	// FIXME: if it already has a limiter delegate attached, we should change it
	// Also if maxLength is 0, then we should remove any existing limiter
	// For now, I'm assuming that this will only be called from Init() methods,
	// so limits won't change.
	OSStatus err = HIObjectCreate( kHITextLengthFilterClassID, initEvent, &limiter );
	__Check_noErr( err );
	if ( noErr == err )
		{
		err = HIObjectAddDelegate( vu, limiter, kHIDelegateBefore );
		__Check_noErr( err );
		}
	if ( limiter )
		CFRelease( limiter );
	
	return err;	
}
#endif  // 10.5+
