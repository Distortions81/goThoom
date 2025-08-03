/*
**	Shell_mac.cp		dtslib2
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

#include <AudioToolbox/AudioToolbox.h>

#include <unistd.h>
#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif


/*
**	VDebugStr();
**	SetErrorCode();
**
**	DTSDate
**	DTSApp
**	StDrawEnv
**
**	RunApp();
**	GetMainScreen();
**	GetNumberOfScreens();
**	GetScreenBounds();
**	GetFrameCounter();
**	IsButtonDown();
**	IsButtonStillDown();
**	IsKeyDown();
**	GetAppData();
**	Beep();
**
**	HandleEvent();
**	Mac2DTSModifiers();
**
**	namespace Prefs;
**	CFStringCreateFormatted();
*/


/*
**	Internal Routines
*/
static bool			Do1Event( int );
static void			DoOSEvent();
static int			Mac2DTSKey( int mackey );
static bool			DoMouseDown();
static void			DoSuspend();
static void			DoResume();


/*
**	Global Variables
*/
DTSError			gErrorCode;
DTSApp  *			gDTSApp;			// global instance of the application
DTSFileSpec			gDTSOpenFile;		// global data file to open
bool				bDebugMessages;


/*
**	Private Library Variables
*/
//DTSRect			gGrowLimit;
EventRecord			gEvent;
//bool				(*gFreeProc)();
long				gSleepTime;
bool				gSuspended;
bool				gNetworking;
bool				gMenuHilited;

// machine capabilities
OSFeatures			gOSFeatures;


/*
**	Internal Variables
*/


/*
**	VDebugStr()
**
**	print out a diagnostic string
*/
void
VDebugStr( const char * format, ... )
{
	if ( not bDebugMessages )
		return;
	
#if TARGET_API_MAC_OSX
	va_list params;
	
	va_start( params, format );
	vfprintf( stderr, format, params );
	va_end( params );
	fputc( '\n', stderr );
	
	// break to debugger
	//	pthread_kill( pthread_self(), SIGINT );
	raise( SIGINT );
	
#else  // OS 8
	
	Str255 buffer;
	va_list params;
	
	va_start( params, format );
	int len = vsnprintfx( (char *) buffer + 1, sizeof buffer - 1, format, params );
	buffer[ 0 ] = static_cast<uchar>( len );
	va_end( params );
	
	DebugStr( buffer );
#endif  // OS X
}


/*
**	SetErrorCode()
*/
void
SetErrorCode( DTSError result )
{
	if ( gErrorCode == noErr
	&&   result     != noErr )
		{
		gErrorCode = result;
		if ( result < noErr )
			VDebugStr( "ErrorCode %d set.", (int) gErrorCode );
		}
}


/*
**	DTSDate::Get()
**
**	set to the current date & time
*/
void
DTSDate::Get()
{
	// get the current date/time, in seconds since 1/1/1904 (Classic Mac epoch)
	dateInSeconds = static_cast<ulong>(
		CFAbsoluteTimeGetCurrent() + kCFAbsoluteTimeIntervalSince1904 );
	
	// convert the seconds to the date
	SetFromSeconds();
}


// CFGregorianDate & friends are deprecated from 10.9 on.  This switch enables an alternate
// code-path that uses CFCalendar instead.
#define USE_CF_CALENDAR		0


/*
**	DTSDate::SetFromYMDHMS()
**
**	set the seconds from the date
*/
void
DTSDate::SetFromYMDHMS()
{
	// convert the date to seconds
#if USE_CF_CALENDAR
	if ( CFCalendarRef cal = CFCalendarCreateWithIdentifier( kCFAllocatorDefault,
								kCFGregorianCalendar ) )
		{
		CFAbsoluteTime at = 0;
		if ( CFCalendarComposeAbsoluteTime( cal, &at, "yMdHms",
				dateYear, dateMonth, dateDay, dateHour, dateMinute, dateSecond ) )
			{
			dateInSeconds = static_cast<ulong>( at + kCFAbsoluteTimeIntervalSince1904 );
			}
		
		CFRelease( cal );
		}
#else  // ! USE_CF_CALENDAR
	CFGregorianDate gd;
	gd.year   = dateYear;
	gd.month  = static_cast<SInt8>( dateMonth );
	gd.day    = static_cast<SInt8>( dateDay );
	gd.hour   = static_cast<SInt8>( dateHour );
	gd.minute = static_cast<SInt8>( dateMinute );
	gd.second = dateSecond;
	
	if ( CFTimeZoneRef tz = CFTimeZoneCopyDefault() )
		{
		// get seconds since 1904
		CFAbsoluteTime at = CFGregorianDateGetAbsoluteTime( gd, tz )
						  + kCFAbsoluteTimeIntervalSince1904;
		dateInSeconds = static_cast<ulong>( at );
		CFRelease( tz );
		}
#endif  // USE_CF_CALENDAR
	
	// now reconvert the seconds back into a date
	// to normalize the date fields, and determine the day of the week
	SetFromSeconds();
}


/*
**	DTSDate::SetFromSeconds()
**
**	set the date from the seconds
*/
void
DTSDate::SetFromSeconds()
{
	// convert the seconds to the date
#if USE_CF_CALENDAR
	if ( CFCalendarRef cal = CFCalendarCreateWithIdentifier( kCFAllocatorDefault,
								kCFGregorianCalendar ) )
		{
		int year, month, day, hour, minute, second;
		CFAbsoluteTime at = dateInSeconds - kCFAbsoluteTimeIntervalSince1904;
		if ( CFCalendarDecomposeAbsoluteTime( cal, at, "yMdHms",
				&year, &month, &day, &hour, &minute, &second ) )
			{
			dateYear = int16_t( year );
			dateMonth = int16_t( month );
			dateDay = int16_t( day );
			dateHour = int16_t( hour );
			dateMinute = int16_t( minute );
			dateSecond = int16_t( second );
			CFIndex dow = CFCalendarGetOrdinalityOfUnit( cal,
								kCFCalendarUnitWeekday, kCFCalendarUnitWeek, at );
			// dow has 1 == Sunday. We want 1 == monday
			if ( --dow <= 0 )
				dow += 7;
			dateDayOfWeek = static_cast<int16_t>( dow );
			}
		CFRelease( cal );
		}
#else  // ! USE_CF_CALENDAR
	if ( CFTimeZoneRef tz = CFTimeZoneCopyDefault() )
		{
		CFAbsoluteTime t0 = dateInSeconds - kCFAbsoluteTimeIntervalSince1904;
		CFGregorianDate gd = CFAbsoluteTimeGetGregorianDate( t0, tz );
		
		dateYear   = static_cast<int16_t>( gd.year );
		dateMonth  = gd.month;
		dateDay    = gd.day;
		dateHour   = gd.hour;
		dateMinute = gd.minute;
		dateSecond = static_cast<int16_t>( gd.second );
		dateDayOfWeek = static_cast<int16_t>( CFAbsoluteTimeGetDayOfWeek( t0, tz ) );
		
		CFRelease( tz );
		}
#endif  // USE_CF_CALENDAR
}


/*
**	InspectOSFeatures()
**
**	see what we've got to work with
*/
static void
InspectOSFeatures()
{
	// start with a clean slate
	memset( &gOSFeatures, 0, sizeof gOSFeatures );
	
	// get OS version
	SInt32 data;
//	OSStatus err = ::Gestalt( gestaltSystemVersion, &gOSFeatures.osVersion );
	OSStatus err = ::Gestalt( gestaltSystemVersionMajor, &data );
	__Check_noErr( err );
	if ( noErr == err )
		gOSFeatures.osVersionMajor = static_cast<SInt16>( data );
	err = ::Gestalt( gestaltSystemVersionMinor, &data );
	__Check_noErr( err );
	if ( noErr == err )
		gOSFeatures.osVersionMinor = static_cast<SInt16>( data );
	err = ::Gestalt( gestaltSystemVersionBugFix, &gOSFeatures.osVersionBugFix );
	__Check_noErr( err );
	
	// a bunch of features that were conditionally present in pre-OSX, but now we know we have 'em
//	gOSFeatures.hasCarbonEvents =			// Carbon Events
//	gOSFeatures.hasAquaMenus =				// Aqua menu layout
//	gOSFeatures.hasHFSPlus = true;			// HFSPlus
	
	// Appearance Manager
//	err = Gestalt( gestaltAppearanceVersion, &data );
//	gOSFeatures.appearanceVersion = data;
//	__Check_noErr( err );
	
	// Thread Manager (or Multiprocessing)
//	gOSFeatures.hasThreadMgr = true;	// MPLibraryIsLoaded() ??
	
	// QuickTime
	err = Gestalt( gestaltQuickTimeVersion, &data );
	if ( noErr == err )
		gOSFeatures.hasQuickTime = (data != 0);
}


/*
**	DTSApp
*/
DTSDefineImplementFirmNoCopy(DTSAppPriv)


/*
**	DTSAppPriv::DTSAppPriv()
*/
DTSAppPriv::DTSAppPriv() :
	appCreator(),
	appResFile( ::CurResFile() ),
	appFrontWin()
{
	UInt32 aType;
	CFBundleGetPackageInfo( CFBundleGetMainBundle(), &aType, &appCreator );
}


/*
**	DTSAppPriv::~DTSAppPriv()
*/
DTSAppPriv::~DTSAppPriv()
{
}


/*
**	DTSApp::DTSApp()
*/
DTSApp::DTSApp()
{
	DTSAppPriv * p = priv.p;
	if ( not p )
		return;
	
	gDTSApp = this;
	
	// initialize the random number seed
	UInt32 randomTime = static_cast<UInt32>( CFAbsoluteTimeGetCurrent() * 1000000 );
	SetRandSeed( randomTime );
	srandom( randomTime );
	
	InspectOSFeatures();
	InitAppleEvents();
	
	// init the variables
	gSuspended = false;
	
	// this is (min size, max size), not ltbr:
//	gGrowLimit.Set( 48 + 16, 48 + 16, 0x7FFF, 0x7FFF );
	
	gSleepTime = 0;
	
	// initialize the current volume and directory and the app signature
	SetErrorCode( InitCurVolDir() );
}


/*
**	DTSApp::~DTSApp()
**
**	We have quit
*/
DTSApp::~DTSApp()
{
	gDTSApp = nullptr;
}


/*
**	DTSApp::SetSleepTime()
**
**	set how long to sleep while getting events
**	returns previous value
*/
long
DTSApp::SetSleepTime( long newTime )
{
	long oldTime = gSleepTime;
	gSleepTime = newTime;
	return oldTime;
}


/*
**	ActivateWindows()
**
**	this only ensures that the appFrontWin is correct;
**	it doesn't do any actual activation (cf DoActivate() below).
**	Returns true if the new appFrontWin is non-null (and has changed).
*/
bool
ActivateWindows()
{
	DTSApp * app = gDTSApp;
	if ( not app )
		{
		VDebugStr( "Application needs a DTSApp." );
		return false;
		}
	
	DTSAppPriv * p = app->priv.p;
	if ( not p )
		return false;
	
	// update the appFrontWin
	DTSWindow * front = p->appFrontWin;
	DTSWindow * dw = FindDTSWindow( ActiveNonFloatingWindow() );
	if ( front != dw )
		{
		// the old front is wrong
		p->appFrontWin = dw;
		return dw != nullptr;
		}
	
	return false;
}

#if defined(MAC_OS_X_VERSION_MAX_ALLOWED) && MAC_OS_X_VERSION_MAX_ALLOWED < 1080
# define QD_OK	1
#else
# undef QD_OK
#endif


/*
**	StDrawEnv::StDrawEnv()
**
**	constructor from a GrafPtr, which, in non-Carbon, is also the base type
**	for WindowRefs and DialogRefs.
*/
StDrawEnv::StDrawEnv( GrafPtr port, GDHandle device )
{
#if QD_OK
	::GetGWorld( &envPort, &envDevice );
	::SetGWorld( CGrafPtr( port ), device );
#else
# pragma unused( port, device )
#endif
}


/*
**	StDrawEnv::StDrawEnv()
**
**	constructor from WindowRef
*/
// in carbon GrafPtr == CGrafPtr == OpaqueGrafPtr != WindowRef != DialogRef
StDrawEnv::StDrawEnv( WindowRef window, GDHandle device )
{
#if QD_OK
	GrafPtr port = ::GetWindowPort( window );
	::GetGWorld( &envPort, &envDevice );
	::SetGWorld( port, device );
#else
# pragma unused( window, device )
#endif
}


/*
**	StDrawEnv::StDrawEnv()
**
**	constructor from DialogRef
*/
StDrawEnv::StDrawEnv( DialogRef dialog, GDHandle device )
{
#if QD_OK
	GrafPtr port = ::GetDialogPort( dialog );
	::GetGWorld( &envPort, &envDevice );
	::SetGWorld( port, device );
#else
# pragma unused( dialog, device )
#endif
}


/*
**	StDrawEnv::StDrawEnv()
**
**	constructor from DTSView *
*/
StDrawEnv::StDrawEnv( const DTSView * view ) : envPort( nullptr ), envDevice( nullptr )
{
#if QD_OK
	DTSViewPriv * p = nullptr;
	if ( view )
		p = view->priv.p;
	if ( p )
		{
		::GetGWorld( &envPort, &envDevice );
		::SetGWorld( (CGrafPtr) p->viewPort, p->viewDevice );
		}
#else
# pragma unused( view )
#endif
}


/*
**	StDrawEnv::~StDrawEnv()
**
**	destructor
*/
StDrawEnv::~StDrawEnv()
{
#if QD_OK
	::SetGWorld( envPort, envDevice );
#endif
}


/*
**	HiliteAllControls()
**
**	activate or deactivate all the controls in the window, all at once,
**	by [de-]activating the root content view
*/
static void
HiliteAllControls( WindowRef window, bool bActive )
{
#if 1
	HIViewRef root = nullptr;
	if ( noErr == HIViewFindByID( HIViewGetRoot( window ), kHIViewWindowContentID, &root ) )
		__Verify_noErr( HIViewSetActivated( root, bActive ) );
#else
	ControlRef root = nullptr;
	OSStatus err = ::GetRootControl( window, &root );
	if ( noErr == err && root )
		{
		if ( bActive )
			err = ::ActivateControl( root );
		else
			err = ::DeactivateControl( root );
		}
	SetErrorCode( err );
#endif
}


/*
**	RunApp()
*/
void
RunApp()
{
	if ( not Do1Event( everyEvent ) )
		{
		if ( not gSuspended )
			gDTSApp->Idle();
		}
	
#if 0	// disabled music
	IdleMusic();
#endif
}


/*
**	Do1Event()
*/
bool
Do1Event( int mask )
{
	// make sure the menu bar is not hilighted
	if ( gMenuHilited )
		{
		::HiliteMenu( 0 );
		gMenuHilited = false;
		}
	
	// we can't mask out suspend/resume events
	mask |= osMask;
	
	// ugly hack here
	// return false if no pending events
	// however, the client depends on a steady stream of idle events
	// so if we sleep too long, we have to synthesize fake idles.
	// So, we report one fake idle event after each real (non-idle) event.
	// However, the non-client apps would rather NOT get those fake idles.
	static bool bLastEventWasReal;
	if ( bLastEventWasReal
	&&   gDTSApp
	&&  'CLc\xC6' == gDTSApp->GetSignature() )
		{
		bLastEventWasReal = false;
		return false;
		}
	
	UInt32 delay = gSleepTime;
	if ( ( gSuspended || gNetworking )
	&&	 0 == delay )
		{
		delay = 1;
		}
	
	bLastEventWasReal = ::WaitNextEvent( mask, &gEvent, delay, nullptr );
	
	// handle the event
	HandleEvent();
	
	return bLastEventWasReal;
}


/*
**	HandleEvent()
*/
void
HandleEvent()
{
//	bool bEventHandled = false;
	int what = gEvent.what;
	
	switch ( what )
		{
		case nullEvent:
			break;
		
		case mouseDown:
			/* bEventHandled = */ DoMouseDown();
			break;
		
		case mouseUp:
			// do nothing
			break;
		
		case keyDown:
		case autoKey:
			{
			uint modifiers = Mac2DTSModifiers( &gEvent );
			int ch = Mac2DTSKey( gEvent.message );
			/* bEventHandled = */ gDTSApp->KeyStroke( ch, modifiers );
			} break;
		
		case keyUp:
			// do nothing
			break;
		
		case updateEvt:
			DoUpdate();
			break;
		
		case activateEvt:
			{
			bool activating = (gEvent.modifiers & activeFlag) != 0;
			WindowRef w = WindowRef( gEvent.message );
			DoActivate( w, activating );
			}
			break;
		
		case osEvt:
			DoOSEvent();
//			bEventHandled = true;
			break;
		
		case kHighLevelEvent:
			(void) ::AEProcessAppleEvent( &gEvent );
//			bEventHandled = true;
			break;
		
		default:
// turned this off because MS-Office 2001 apps generate spurious
// 'modifierDown' events, of type 13, even if they are in the background
//			VDebugStr( "Received unknown event (%d).", int( gEvent.what ) );
			break;
		}
}


/*
**	DoMouseDown()
*/
bool
DoMouseDown()
{
	DTSPoint where( gEvent.where.h, gEvent.where.v );
	uint modifiers = Mac2DTSModifiers( &gEvent );
	
	// give the app first crack at this event
	if ( gDTSApp->Click( where, gEvent.when, modifiers ) )
		return true;
	
	// otherwise, do standard processing
	WindowRef window;
	WindowPartCode partCode = ::FindWindow( gEvent.where, &window );
	
	OSStatus err = noErr;
	
	// we handle only clicks in menus...
	if ( inMenuBar == partCode )
		{
		long cmd = ::MenuSelect( *DTS2Mac(&where) );
		gMenuHilited = true;
		if ( HiWord(cmd) )
			{
			if ( DTSMenu * menu = gDTSMenu )
				{
				// for selections in the help menu,
				// offset the cmd, to ignore any system-provided items
				if ( kHMHelpMenuID == HiWord(cmd) )
					cmd -= menu->GetHelpMenuOffset();
					
				menu->DoCmd( cmd );
				}
			}
		return true;
		}
	
	// ...  or in and around our own windows
	if ( DTSWindow * found = FindDTSWindow( window ) )
		{
		SInt32 proxyCmd;
		
		switch ( partCode )
			{
			case inStructure:
			case inContent:
				DTSWindow_MouseDown( *found, where, gEvent.when, modifiers );
				break;
			
			case inDrag:
				if ( found == gDTSApp->GetFrontWindow()
				&&	 ::IsWindowPathSelectClick( window, &gEvent ) )
					{
					WindowPathSelect( window, nullptr, &proxyCmd );
					break;
					}
				
				DTSWindow_Drag( *found, where, gEvent.modifiers & cmdKey );
				break;
			
			case inGrow:
				DTSWindow_Grow( *found, where );
				break;
			
			case inGoAway:
				DTSWindow_GoAway( *found, where );
				break;
			
			case inZoomIn:
			case inZoomOut:
				DTSWindow_Zoom( *found, where, partCode );
				break;
			
			case inCollapseBox:
				// don't need to do anything
				break;
			
			case inProxyIcon:
				if ( ::IsWindowPathSelectClick( window, &gEvent ) )
					::WindowPathSelect( window, nullptr, &proxyCmd );
				else
					{
					err = ::TrackWindowProxyDrag( window, *DTS2Mac(&where) );
					if ( errUserWantsToDragWindow == err )
						DTSWindow_Drag( *found, where, gEvent.modifiers & cmdKey );
					}
				break;
			}
		
		return true;
		}
	
	// we don't handle that type of click
	return false;
}


/*
**	Mac2DTSModifiers()
**
**	as below, but only for keyboard modifiers (as might be included in a CarbonEvent)
*/
uint
Mac2DTSModifiers( UInt32 macMods )
{
	uint modifiers = 0;
	if ( macMods & (shiftKey | rightShiftKey) )
		modifiers |= kKeyModShift;
	if ( macMods & (controlKey | rightControlKey) )
		modifiers |= kKeyModControl;
	if ( macMods & cmdKey )
		modifiers |= kKeyModMenu;
	if ( macMods & (optionKey | rightOptionKey) )
		modifiers |= kKeyModOption;
	if ( macMods & alphaLock )
		modifiers |= kKeyModCapsLock;
	
	// does NOT set: kKeyModRepeat, kKeyModNumpad
	return modifiers;
}


/*
**	Mac2DTSModifiers()
*/
uint
Mac2DTSModifiers( const EventRecord * event )
{
	uint modifiers = Mac2DTSModifiers( event->modifiers );
	if ( event->what == autoKey )
		modifiers |= kKeyModRepeat;
	
	// check for numpad keys
	int code = (event->message >> 8) & 0x0FF;
	switch ( code )
		{
		case 0x47:	//	clear
		case 0x51:	//	=
		case 0x4b:	//	/
		case 0x43:	//	*
		case 0x59:	//	7
		case 0x5b:	//	8
		case 0x5c:	//	9
		case 0x4e:	//	-
		case 0x56:	//	4
		case 0x57:	//	5
		case 0x58:	//	6
		case 0x45:	//	+
		case 0x53:	//	1
		case 0x54:	//	2
		case 0x55:	//	3
					// we're ignoring Enter, 0x4C -- purposely, I guess...
		case 0x52:	//	0
		case 0x41:	//	.
			modifiers |= kKeyModNumpad;
			break;
		}
	
	return modifiers;
}


/*
**	Mac2DTSKey()
**
**	map the function keys
*/
int
Mac2DTSKey( int mackey )
{
	int dtskey = mackey & 0x0FF;
	if ( kFunctionKeyCharCode == dtskey )
		{
		switch ( (mackey >> 8) & 0x0FF )
			{
			case 0x7A:	dtskey = kF1Key;	break;
			case 0x78:	dtskey = kF2Key;	break;
			case 0x63:	dtskey = kF3Key;	break;
			case 0x76:	dtskey = kF4Key;	break;
			case 0x60:	dtskey = kF5Key;	break;
			case 0x61:	dtskey = kF6Key;	break;
			case 0x62:	dtskey = kF7Key;	break;
			case 0x64:	dtskey = kF8Key;	break;
			case 0x65:	dtskey = kF9Key;	break;
			case 0x6D:	dtskey = kF10Key;	break;
			case 0x67:	dtskey = kF11Key;	break;
			case 0x6F:	dtskey = kF12Key;	break;
			case 0x69:	dtskey = kF13Key;	break;
			case 0x6B:	dtskey = kF14Key;	break;
			case 0x71:	dtskey = kF15Key;	break;
			case 0x6A:	dtskey = kF16Key;	break;
			}
		}
	
	return dtskey;
}


/*
**	DTSApp::KeyStroke()
*/
bool
DTSApp::KeyStroke( int ch, uint modifiers )
{
	bool bResult = false;
	
	// the calling app can define whether or not
	// the menu command keys repeat while the key is held down
	uint testFlags = kKeyModMenu | kKeyModRepeat;
	DTSMenu * menu = gDTSMenu;
	if ( menu && menu->GetAutoRepeat() )
		testFlags = kKeyModMenu;
	
	// test if menu key
	if ( (modifiers & testFlags) == kKeyModMenu )
		{
		int cmd;
		if ( ch == Mac2DTSKey( gEvent.message & charCodeMask ) )
			cmd = ::MenuEvent( &gEvent );
		else
			cmd = ::MenuKey( ch );
		
		gMenuHilited = true;
		if ( cmd >> 16 )
			{
			if ( menu )
				{
				// for selections in the help menu,
				// offset the cmd, to ignore any system-provided items
				if ( kHMHelpMenuID == HiWord( cmd ) )
					cmd -= menu->GetHelpMenuOffset();
				
				menu->DoCmd( cmd );
				bResult = true;
				}
			}
		}
	
	return bResult;
}


/*
**	DTSApp::Suspend()
*/
void
DTSApp::Suspend()
{
}


/*
**	DTSApp::Resume()
*/
void
DTSApp::Resume()
{
}


/*
**	DTSApp::GetSignature()
*/
uint32_t
DTSApp::GetSignature() const
{
	const DTSAppPriv * p = priv.p;
	return p ? p->appCreator : 0;
}


/*
**	DTSApp::GetResFile()
*/
int
DTSApp::GetResFile() const
{
	const DTSAppPriv * p = priv.p;
	return p ? p->appResFile : 0;
}


/*
**	DTSApp::GetFrontWindow()
*/
DTSWindow *
DTSApp::GetFrontWindow() const
{
	const DTSAppPriv * p = priv.p;
	return p ? p->appFrontWin : nullptr;
}


/*
**	DoUpdate()
*/
void
DoUpdate()
{
	WindowRef window = WindowRef( gEvent.message );
	if ( not window )
		{
		// I have observed update events for these pseudo windows, although I have
		// no idea what caused them. Perhaps changing screen resolutions or themes
		// can trigger them? Or if the menubar gets hidden somehow?
		// Anyway, if such update events remain unhandled, then updates are blocked
		// for all other windows too, which is bad.
		::BeginUpdate( window );
		::EndUpdate( window );
		}
	else
	if ( kDialogWindowKind == ::GetWindowKind( window ) )
		{
		::BeginUpdate( window );
		::UpdateDialog( ::GetDialogFromWindow( window ), nullptr );
		::EndUpdate( window );
		}
	else
	if ( DTSWindow * found = FindDTSWindow( window ) )
		DTSWindow_Update( *found );
}


/*
**	DoActivate()
*/
void
DoActivate( WindowRef macWin, bool activating )
{
	if ( not gDTSApp )
		{
		VDebugStr( "Application needs a DTSApp." );
		return;
		}
	
	DTSAppPriv * ap = gDTSApp->priv.p;
	if ( not ap )
		return;
	
	DTSWindow * win = FindDTSWindow( macWin );

	// if it's not a DTSWindow, we have to assume that it can take care
	// of itself. Otherwise we do the work here.
	if ( win )
		{
		HiliteAllControls( macWin, activating );
		UpdateControls( macWin, nullptr );	// ?fix scroll-bar-not-updating glitch?
		
		if ( activating )
			{
//			DTSWindow_Hilite( *win );
			
			// become the new appFrontWin (unless it's a floater)
			DTSWindowPriv * wp = win->priv.p;
			if ( wp && not wp->winIsFloater )
				ap->appFrontWin = win;
			
			win->Activate();
			}
		else
			{
			// deactivating
//			DTSWindow_Hilite( *win );
			
			if ( win == ap->appFrontWin )
				ap->appFrontWin = nullptr;
			
			win->Deactivate();
			}
		}
}


/*
**	DoOSEvent()
*/
void
DoOSEvent()
{
	int suspendResumeEvent = gEvent.message >> 24;
	
	switch ( suspendResumeEvent )
		{
		case suspendResumeMessage:
			if ( gEvent.message & activeFlag )
				DoResume();
			else
				DoSuspend();
			break;
		
		case mouseMovedMessage:
			break;
		
		default:
#ifdef DEBUG_VERSION
			VDebugStr( "Unknown OSEvent" );
#endif
			break;
		}
}


/*
**	DoSuspend()
*/
void
DoSuspend()
{
	gSuspended = true;
	
	WindowRef firstWin = GetWindowList();
	
	// chase down the window list, hiding floaters as we go
	for ( WindowRef test = firstWin; test; test = ::GetNextWindow( test ) )
		{
		if ( kFloaterWinKind == ::GetWindowKind( test ) )
			{
			if ( DTSWindow * found = FindDTSWindow( test ) )
				{
				if ( DTSWindowPriv * p = found->priv.p )
					p->winSaveVisible = ::IsWindowVisible( test );
				
				found->Hide();
				}
			else
				::HideWindow( test );
			}
		}
	
	// suspend the app's front window
	DTSApp * app = gDTSApp;
	if ( DTSWindow * front = app->GetFrontWindow() )
		{
		WindowRef window = DTS2Mac( front );
		::HiliteWindow( window, false );
		HiliteAllControls( window, false );
		front->Deactivate();
		}
	
	app->Suspend();
}


/*
**	DoResume()
*/
void
DoResume()
{
	gSuspended = false;
	
//	::InitCursor();
	__Verify_noErr( SetThemeCursor( kThemeArrowCursor ) );
	
	// start at the top of the window list so that we see
	// _all_ windows, visible or not.
	
	WindowRef firstWin = GetWindowList();
	
	// reveal any floaters that were showing before the suspend
	for ( WindowRef test = firstWin; test; test = ::GetNextWindow( test ) )
		{
		if ( kFloaterWinKind == ::GetWindowKind( test ) )
			{
			if ( DTSWindow * found = FindDTSWindow( test ) )
				{
				DTSWindowPriv * p = found->priv.p;
				if ( p  &&  p->winSaveVisible )
					found->Show();
				}
			else
				::ShowWindow( test );
			}
		}
	
	// activate the front non-floater
	DTSApp * app = gDTSApp;
	if ( DTSWindow * front = app->GetFrontWindow() )
		{
		WindowRef window = DTS2Mac(front);
		HiliteAllControls( window, true );
		front->Activate();
		}

	// resume the app
	app->Resume();
}


/*
**	GetMainScreen()
*/
void
GetMainScreen( DTSRect * bounds )
{
	CGRect box = CGDisplayBounds( CGMainDisplayID() );
	
	// beware of flipped y-coordinates
	bounds->rectTop    = static_cast<DTSCoord>( CGRectGetMinY( box ) + GetMBarHeight() );
	bounds->rectLeft   = static_cast<DTSCoord>( CGRectGetMinX( box ) );
	bounds->rectBottom = static_cast<DTSCoord>( CGRectGetMaxY( box ) );
	bounds->rectRight  = static_cast<DTSCoord>( CGRectGetMaxX( box ) );
}


/*
**	GetNumberOfScreens()
*/
int
GetNumberOfScreens()
{
	CGDisplayCount numScreens = 0;
	__Verify_noErr( CGGetActiveDisplayList( 1, nullptr, &numScreens ) );
	return numScreens;
}


/*
**	GetScreenBounds()
*/
void
GetScreenBounds( int screenNumber, DTSRect * bounds )
{
	if ( screenNumber <= GetNumberOfScreens() )
		{
		// let's assume that nobody can possibly ever have more than, oh, 16 displays at once
		enum { kMaxScreens = 16 };
		
		CGDisplayCount numToGet = screenNumber, numGot;
		CGDirectDisplayID displays[ kMaxScreens ];
		
		if ( numToGet >= kMaxScreens )
			{
#ifdef DEBUG_VERSION
			VDebugStr( "too many displays" );
#endif
			numToGet = kMaxScreens - 1;
			}
		
		CGDisplayErr err = CGGetActiveDisplayList( numToGet + 1, displays, &numGot );
		__Check_noErr( err );
		if ( noErr == err )
			{
			CGRect box = CGDisplayBounds( displays[ numToGet ] );
			
			// beware of flipped Y-coords
			bounds->rectTop    = static_cast<DTSCoord>( CGRectGetMinY( box ) );
			bounds->rectLeft   = static_cast<DTSCoord>( CGRectGetMinX( box ) );
			bounds->rectBottom = static_cast<DTSCoord>( CGRectGetMaxY( box ) );
			bounds->rectRight  = static_cast<DTSCoord>( CGRectGetMaxX( box ) );
			if ( displays[ numToGet ] == CGMainDisplayID() )
				bounds->rectTop += GetMBarHeight();
			return;
			}
		}
	
	bounds->Set( 0, 0, 0, 0 );
}


/*
**	GetFrameCounter()
*/
ulong
GetFrameCounter()
{
//	return TickCount();
	return EventTimeToTicks( GetCurrentEventTime() );
}


/*
**	IsButtonDown()
*/
bool
IsButtonDown()
{
	if ( gSuspended )
		return false;
	
	enum { kPrimaryButtonMask = 1U << 0 };	// only care about main button, #0
	return (GetCurrentEventButtonState() & kPrimaryButtonMask) != 0;
}


/*
**	IsButtonStillDown()
*/
bool
IsButtonStillDown()
{
	if ( gSuspended )
		return false;
	
	return ::WaitMouseUp();
}


/*
**	IsKeyDown()
*/
bool
IsKeyDown( int ch )
{
#if 0	// this will be nice once we use RunApplicationEventLoop()
	
	// take a fast shortcut for modifier keys
	if ( ch >= kMenuModKey && ch <= kCapsLockModKey )
		{
		const UInt16 kModifierMap[] =
			{
			cmdKey,								// kMenuModKey
			optionKey     | rightOptionKey,		// kOptionModKey
			shiftKeyBit   | rightShiftKey,		// kShiftModKey
			controlKeyBit | rightControlKey,	// kControlModKey
			alphaLockBit						// kCapsLockModKey
			};
		
		UInt32 testBit = kModifierMap[ ch - kMenuModKey ];
		UInt32 mods;
		if ( not gSuspended )
			mods = GetCurrentEventKeyModifiers();
		else
			/* if we're suspended, the queue-sync'ed state is stale,
			   so ask the hardware directly (slow) -- see <CarbonEventsCore.h> */
			mods = GetCurrentKeyModifiers();
		
		return not (mods & testBit);
		}
#endif  // 0
	
	// FIXME: This could fail really badly for non-US keyboards, because it totally ignores
	// the workings of the KCHR/uchr mechanism(s).
	
	// map ASCII to virtual keycodes
	const signed char kbdMap1[] = {
		/*			 00  01  02  03  04  05  06  07  08  09  0A  0B  0C  0D  0E  0F */
		/* 00 */	 -1,115, -1, 76,119,114, -1, -1, 51, 48, -1,116,121, 36, -1, -1,
		//			 home	   entr end  help			delt tab	   pgup pgdn ret
		//		0xFF,0x73,0xFF,0x4C,0x77,0x72,0xFF,0xFF,0x33,0x30,0xFF,0x74,0x79,0x24,0xFF,0xFF,
		
		/* 10 */	 -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 53,123,124,126,125,
		//															   esc  lt   rt   up   dn
		//		0xFF,0xFF,0xFF,0xFF,0xFF,0xFF,0xFF,0xFF,0xFF,0xFF,0xFF,0x35,0x7B,0x7C,0x7E,0x7D,
		
		/* 20 */	 49, 18, 39, 20, 21, 23, 26, 39, 25, 29, 28, 24, 43, 27, 47, 44,
		//		spc  "!"  "\"" "#"  "$"  "%"  "&"  "'"  "("  ")"  "*"  "+"  ","  "-"  "."  "/"
		//		0x31,0x12,0x27,0x14,0x15,0x17,0x1A,0x27,0x19,0x1D,0x1C,0x18,0x2B,0x1B,0x2F,0x2C,
		
		/* 30 */	 29, 18, 19, 20, 21, 23, 22, 26, 28, 25, 41, 41, 43, 24, 47, 44,
		//		"0"  "1"  "2"  "3"  "4"  "5"  "6"  "7"  "8"  "9"  ":"  ";"  "<"  "="  ">"  "?"
		//		0x1D,0x12,0x13,0x14,0x15,0x17,0x16,0x1A,0x1C,0x19,0x29,0x29,0x2B,0x18,0x2F,0x2C,
		
		/* 40 */	 19,  0, 11,  8,  2, 14,  3,  5,  4, 34, 38, 40, 37, 46, 45, 31,
		//		"@"  "A"  "B"  "C"  "D"  "E"  "F"  "G"  "H"  "I"  "J"  "K"  "L"  "M"  "N"  "O"
		//		0x13,0x00,0x0B,0x08,0x02,0x0E,0x03,0x05,0x04,0x22,0x26,0x28,0x25,0x2E,0x2D,0x1F,
		
		/* 50 */	 35, 12, 15,  1, 17, 32,  9, 13,  7, 16,  6, 33, 42, 30, 22, 27,
		//		"P"  "Q"  "R"  "S"  "T"  "U"  "V"  "W"  "X"  "Y"  "Z"  "["  "\\" "]"  "^"  "_"
		//		0x23,0x0C,0x0F,0x01,0x11,0x20,0x09,0x0D,0x07,0x10,0x06,0x21,0x2A,0x1E,0x16,0x1B,
		
		/* 60 */	 50,  0, 11,  8,  2, 14,  3,  5,  4, 34, 38, 40, 37, 46, 45, 31,
		//		"`"  "a"  "b"  "c"  "d"  "e"  "f"  "g"  "h"  "i"  "j"  "k"  "l"  "m"  "n"  "o"
		//		0x32,0x00,0x0B,0x08,0x02,0x0E,0x03,0x05,0x04,0x22,0x26,0x28,0x25,0x2E,0x2D,0x1F,
		
		/* 70 */	 35, 12, 15,  1, 17, 32,  9, 13,  7, 16,  6, 33, 42, 30, 50,117,
		//		"p"  "q"  "r"  "s"  "t"  "u"  "v"  "w"  "x"  "y"  "z"  "{"  "|"  "}"  "~"  del
		//		0x23,0x0C,0x0F,0x01,0x11,0x20,0x09,0x0D,0x07,0x10,0x06,0x21,0x2A,0x1E,0x32,0x75,
		
		/* 80 */	 55, 58, 56, 59, 57,122,120, 99,118, 96, 97, 98,100,101,109,103,
		//		cmd  opt  shft ctrl caps "F1" "F2" "F3" "F4" "F5" "F6" "F7" "F8" "F9" "F10""F11"
		//		0x37,0x3A,0x38,0x3B,0x39,0x7A,0x78,0x63,0x76,0x60,0x61,0x62,0x64,0x65,0x6D,0x75,
		
		/* 90 */	111,105,107,113,106
		//		F12  F13  F14  F15  F16
		//		0x6F,0x69,0x6B,0x71,0x70
		};
	
	// same thing, but with numeric keypad
	const signed char kbdMap2[] = {
		/*			 00  01  02  03  04  05  06  07  08  09  0A  0B  0C  0D  0E  0F */
		/* 00 */	 -1,115, -1, 76,119,114, -1, -1, 51, 48, -1,116,121, 36, -1, -1,
		/* 10 */	 -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 53,123,124,126,125,
		/* 20 */	 49, 18, 39, 20, 21, 23, 26, 39, 25, 29, 67, 69, 43, 78, 65, 75,
		//														  n*   n+		 n-   n.   n/
		//		0x31,0x12,0x27,0x14,0x15,0x17,0x1A,0x27,0x19,0x1D,0x43,0x45,0x2B,0x4E,0x41,0x4B,
		
		/* 30 */	 82, 83, 84, 85, 86, 87, 88, 89, 91, 92, 41, 41, 43, 81, 47, 44,
		//		n0	 n1	  n2   n3   n4   n5   n6   n7   n8   n9					 n=
		//		0x52,0x53,0x54,0x55,0x56,0x57,0x58,0x59,0x5B,0x5C,0x29,0x29,0x2B,0x51,0x2B,0x2C,
		
		/* 40 */	 19,  0, 11,  8,  2, 14,  3,  5,  4, 34, 38, 40, 37, 46, 45, 31,
		/* 50 */	 35, 12, 15,  1, 17, 32,  9, 13,  7, 16,  6, 33, 42, 30, 22, 27,
		/* 60 */	 50,  0, 11,  8,  2, 14,  3,  5,  4, 34, 38, 40, 37, 46, 45, 31,
		/* 70 */	 35, 12, 15,  1, 17, 32,  9, 13,  7, 16,  6, 33, 42, 30, 50,117,
		/* 80 */	 55, 58, 56, 59, 57,122,120, 99,118, 96, 97, 98,100,101,109,103,
		/* 90 */	111,105,107,113,106
		};
	
	bool result = false;
	
	// remap "synthetic" DTS keys from 0x100-0x114 down to 0x80-0x94, so the above tables
	// can be contiguous.
	if ( ch >= 0x0080 )
		{
		if ( ch >= kMenuModKey
		&&   ch <= kF16Key )
			ch += 0x080 - kMenuModKey;
		else
			ch = -1;
		}
	
	if ( ch >= 0
	&&   ch < (int) sizeof kbdMap1 )
		{
		signed char ch2 = kbdMap1[ ch ];
		if ( ch2 >= 0 )
			{
			int bit = ch2 & 0x07;
			int byt = ch2 >> 3;
			
			KeyMapByteArray keys;
			::GetKeys( * (KeyMap *) &keys );
			
			if ( keys[byt] & ( 1U << bit ) )
				result = true;
			else
				{
				ch2 = kbdMap2[ ch ];
				if ( ch2 >= 0 )
					{
					bit = ch2 & 0x07;
					byt = ch2 >> 3;
					
					if ( keys[byt] & ( 1U << bit ) )
						result = true;
					}
				}
			}
		}
	
	return result;
}


/*
**	DTSTimerPriv::DTSTimerPriv()
**
**	ctor
*/
template<class TimeVal>
DTSTimerPriv<TimeVal>::DTSTimerPriv()
#ifdef DEBUG_VERSION
	: mTimerStarted( false )
#endif
{
}


/*
**	DTSTimerPriv::StartTimer()
**
**	start watching the clock
*/
template<class TimeVal>
void
DTSTimerPriv<TimeVal>::StartTimer()
{
	mStartTime = GetCurrentTime();

#ifdef DEBUG_VERSION
	mTimerStarted = true;
#endif
}


/*
**	DTSTimerPriv::StopTimer()
**
**	return microseconds since this timer object was started.
*/
template<class TimeVal>
ulong
DTSTimerPriv<TimeVal>::StopTimer() const
{
#ifdef DEBUG_VERSION
	// sanity check
	if ( not mTimerStarted )
		{
		VDebugStr( "%s: timer not running.", __FUNCTION__ );
		return 0;
		}
#endif
	
	TimeVal diff;
	TimeVal now = GetCurrentTime();
	
	DiffTimer( diff, now, mStartTime );
	
	ulong result = ConvertToMicrosecs( diff );
	return result;
}


/*
**	DTSTimerPriv::DiffTimer()	[ static ]
**
**	calculate the difference between two absolute time values
*/
template<class TimeVal>
void
DTSTimerPriv<TimeVal>::DiffTimer( TimeVal& diff, const TimeVal& endTime, const TimeVal& startTime )
{
	// simple floating-point subtraction
	diff = endTime - startTime;
}


/*
**	DTSTimerPriv::ConvertToMicrosecs()		[ static ]
**
**	translate an internal time measurement to microseconds
*/
template<class TimeVal>
ulong
DTSTimerPriv<TimeVal>::ConvertToMicrosecs( const TimeVal& diff )
{
	// convert from floating point
	return static_cast<ulong>( diff * 1000000.0 );
}


/*
**	DTSTimerPriv::GetCurrentTime()		[ static ]
**
**	read the current timestamp (in arbitrary units & format)
*/
template<class TimeVal>
TimeVal
DTSTimerPriv<TimeVal>::GetCurrentTime()
{
	return CFAbsoluteTimeGetCurrent();
}

// now we can instantiate the template
template class DTSTimerPriv<DTSTimer_t>;


/*
**	GetAppData()
*/
DTSError
GetAppData( uint32_t type, int resID, void ** ptr, size_t * oSize )
{
	// paranoia
	__Check( ptr );
	if ( not ptr )
		return paramErr;
	
	*ptr = nullptr;
	if ( oSize )
		*oSize = 0;
	
	OSStatus result = noErr;
	Handle hdl = GetResource( type, static_cast<ResID>( resID ) );
	if ( not hdl )
		{
		result = ::ResError();
		if ( noErr == result )
			result = resNotFound;
		SetErrorCode( result );
		return result;
		}
	
	::Size sz = ::GetHandleSize( hdl );
	if ( char * data = NEW_TAG("DTSGetAppData") char[ sz ] )
		{
		memcpy( data, *hdl, sz );
		*ptr  = static_cast<void *>( data );
		if ( oSize )
			*oSize = sz;
		}
	else
		result = memFullErr;
	ReleaseResource( hdl );
	
	return result;
}


/*
**	Beep()
*/
void
Beep()
{
#if MAC_OS_X_VERSION_MIN_REQUIRED >= 1050
//	AudioServicesPlayAlertSound( kSystemSoundID_UserPreferredAlert );
	AudioServicesPlayAlertSound( kUserPreferredAlert );
#else
	AlertSoundPlay();
#endif
}


#pragma mark -
#pragma mark CFPreferences Services Support

namespace Prefs
{
/*
**	Prefs::Set()
**
**	Set an application preference value.
**	If 'value' is NULL, the key is removed.
*/
void
Set( CFStringRef key, CFPropertyListRef value )
{
	__Check( key );
	CFPreferencesSetAppValue( key, value, kCFPreferencesCurrentApplication );
}


/*
**	Prefs::Set()	-- bool specialization
*/
void
Set( CFStringRef key, bool value )
{
	Set( key, value ? kCFBooleanTrue : kCFBooleanFalse );
}


/*
**	Prefs::Set()	-- int specialization
**
**	(for any numeric type that's not an int, you'll either have to cast it to an int --
**	assuming it fits -- or go to CFNumbers. Otherwise the compiler will flag an ambiguous
**	overload between this and the bool version above)
*/
void
Set( CFStringRef key, int value )
{
	CFNumberRef nr = CFNumberCreate( kCFAllocatorDefault, kCFNumberIntType, &value );
	__Check( nr );
	if ( nr )
		{
		Set( key, nr );
		CFRelease( nr );
		}
}


namespace {
/*
**	MyCreateDictFromRect()
**
**	OSX 10.4 doesn't have CGRectCreateDictionaryRepresentation().
**	Here's a home-made reimplementation, if needed
*/
CFDictionaryRef
MyCreateDictFromCGRect( const CGRect& r )
{
#if MAC_OS_X_VERSION_MIN_REQUIRED > 1040
	
	// if Xcode's "Deployment Version" specifies 10.5 or higher -- so that we know this code
	// will NEVER run under 10.4 -- then we can use the new API unconditionally.
	return CGRectCreateDictionaryRepresentation( r );
	
#else
# if MAC_OS_X_VERSION_MAX_ALLOWED > 1040
	
	// if the SDK is new enough, the API _may_ be available at runtime, but we need
	// to do a weak-link test first.
	
	if ( IsCFMResolved( CGRectCreateDictionaryRepresentation ) )
		return CGRectCreateDictionaryRepresentation( r );
# endif  // SDK 10.5+
	
	// we get here if the weak-link test fails, OR if the SDK predates 10.5.
	// Gotta use our hand-rolled version of CGRectCreateDictionaryRepresentation()
	
	// The canonical dictionary keys are these:
	const CFStringRef keys[ 4 ] =
	{
		CFSTR("X"),
		CFSTR("Y"),
		CFSTR("Width"),
		CFSTR("Height")
	};
	
	CFNumberRef values[ 4 ];
	
# if CGFLOAT_DEFINED
	values[ 0 ] = CFNumberCreate( kCFAllocatorDefault, kCFNumberCGFloatType, &r.origin.x );
	values[ 1 ] = CFNumberCreate( kCFAllocatorDefault, kCFNumberCGFloatType, &r.origin.y );
	values[ 2 ] = CFNumberCreate( kCFAllocatorDefault, kCFNumberCGFloatType, &r.size.width );
	values[ 3 ] = CFNumberCreate( kCFAllocatorDefault, kCFNumberCGFloatType, &r.size.height );
# else
	// 10.4 doesn't define type CGFloat, but it's always a 'float' on every
	// platform this code supports. So in principle we don't really need to
	// bother copying each item into the temporary value 'f'... but it makes me feel better.
	float f = r.origin.x;
	values[ 0 ] = CFNumberCreate( kCFAllocatorDefault, kCFNumberFloatType, &f );
	f = r.origin.y;
	values[ 1 ] = CFNumberCreate( kCFAllocatorDefault, kCFNumberFloatType, &f );
	f = r.size.width;
	values[ 2 ] = CFNumberCreate( kCFAllocatorDefault, kCFNumberFloatType, &f );
	f = r.size.height;
	values[ 3 ] = CFNumberCreate( kCFAllocatorDefault, kCFNumberFloatType, &f );
# endif  // CGFLOAT_DEFINED
	
	
	CFDictionaryRef dict = CFDictionaryCreate( kCFAllocatorDefault,
								(const void **) keys, (const void **) values, 4,
								&kCFTypeDictionaryKeyCallBacks,
								&kCFTypeDictionaryValueCallBacks );
	__Check( dict );
	
	// dispose of the numbers (the dictionary has its own retain on them)
	for ( int j = 3; j >= 0; --j )
		if ( values[ j ] )
			CFRelease( values[ j ] );
	
	return dict;
#endif  // MIN_REQUIRED > 1040
}


#if MAC_OS_X_VERSION_MIN_REQUIRED <= 1040
/*
**	GetOneFromDict()
**
**	Helper for DictToRect(), below.
**	extracts one floating point value from a dictionary created by
**	MyCreateDictFromCGRect() [or CGRectCreateDictionaryRepresentation()]
**	returns true if value present and valid; otherwise false.
*/
bool
GetOneFromDict( CFDictionaryRef d, CFStringRef key, float * f )
{
	const void * v( nullptr );
	if ( not CFDictionaryGetValueIfPresent( d, key, &v ) )
		return false;
	if ( CFGetTypeID( v ) != CFNumberGetTypeID() )
		return false;
	
	CFNumberRef n = static_cast<CFNumberRef>( v );
	CFNumberGetValue( n, kCFNumberFloatType, f );
//	CFRelease( n );	// that was a "Get", not a "Copy"
	
	return true;
}
#endif  // <= 10.4


/*
**	DictToRect()
**
**	Inverse of MyCreateDictFromCGRect(), above
*/
bool
DictToRect( CFDictionaryRef d, CGRect& oRect )
{
#if MAC_OS_X_VERSION_MIN_REQUIRED > 1040
	// we KNOW we have this API
	return CGRectMakeWithDictionaryRepresentation( d, &oRect );
#else
# if MAC_OS_X_VERSION_MAX_ALLOWED > 1040
	// we MIGHT have it
	if ( IsCFMResolved( CGRectMakeWithDictionaryRepresentation ) )
		return CGRectMakeWithDictionaryRepresentation( d, &oRect );
# endif  // MAX > 10.4
	
	// we DON'T have it -- must roll our own.
	
	CGRect r = CGRectNull;	// assume the dictionary isn't well-formed
	
	if ( CFGetTypeID( d ) != CFDictionaryGetTypeID() )
		return false;
	
	float f;
	if ( not GetOneFromDict( d, CFSTR("X"), &f ) )
		return false;
	r.origin.x = f;
	
	if ( not GetOneFromDict( d, CFSTR("Y"), &f ) )
		return false;
	r.origin.y = f;
	
	if ( not GetOneFromDict( d, CFSTR("Width"), &f ) )
		return false;
	r.size.width = f;
	
	if ( not GetOneFromDict( d, CFSTR("Height"), &f ) )
		return false;
	r.size.height = f;
	
	oRect = r;
	return true;
#endif  // MIN_REQUIRED > 10.4
}

}	// end inner anon namespace

/*
**	Prefs::Set()	-- CGRect / HIRect specialization
*/
void
Set( CFStringRef key, const CGRect& value )
{
	CFDictionaryRef dr = MyCreateDictFromCGRect( value );
	__Check( dr );
	if ( dr )
		{
		Set( key, dr );
		CFRelease( dr );
		}
}

		
/*
**	Prefs::Copy()
**
**	fetch an application preference value
*/
CFPropertyListRef
Copy( CFStringRef key )
{
	__Check( key );
	return CFPreferencesCopyAppValue( key, kCFPreferencesCurrentApplication );
}


/*
**	Prefs::GetBool()
**
**	return a boolean preference.
**	if the key didn't already exist, returns 'defaultVal' instead.
*/
bool
GetBool( CFStringRef key, bool defaultVal /* = false */ )
{
	__Check( key );
	
	Boolean bHadVal = false;
	bool value = CFPreferencesGetAppBooleanValue( key,
						kCFPreferencesCurrentApplication, &bHadVal );
	
	if ( not bHadVal )
		{
		value = defaultVal;
		// maybe set it now too?
		}
	
	return value;
}


/*
**	Prefs::GetInt()
**
**	returns an integer preference
**	if the key doesn't exist (or can't be parsed as an integer), returns 'defaultVal'
*/
CFIndex
GetInt( CFStringRef key, CFIndex defaultVal /* = 0 */ )
{
	__Check( key );
	
	Boolean bHadVal = false;
	CFIndex value = CFPreferencesGetAppIntegerValue( key,
						kCFPreferencesCurrentApplication, &bHadVal );
	
	if ( not bHadVal )
		{
		value = defaultVal;
		// maybe set it now too?
		}
	
	return value;
}


/*
**	Prefs::GetRect()
*/
CGRect
GetRect( CFStringRef key, const CGRect& defaultValue )
{
	CGRect r;
	bool bHadVal = false;
	if ( CFPropertyListRef plr = Copy( key ) )
		{
		if ( CFDictionaryGetTypeID() == CFGetTypeID( plr )
		&&   DictToRect( static_cast<CFDictionaryRef>( plr ), r ) )
			{
			bHadVal = true;
			}
		CFRelease( plr );
		}
	if ( not bHadVal )
		r = defaultValue;
	
	return r;
}

};	// end namespace "Prefs"


#pragma mark CFString support

/*
**	CFStringCreateFormatted()
**
**	convenience wrapper for CFStringCreateWithFormat()
*/
CFStringRef
CFStringCreateFormatted( CFStringRef fmt, ... )
{
	va_list ap;
	va_start( ap, fmt );
	
	CFStringRef cs = CFStringCreateWithFormatAndArguments( kCFAllocatorDefault, nullptr, fmt, ap );
	__Check( cs );
	
	va_end( ap );
	return cs;
}


