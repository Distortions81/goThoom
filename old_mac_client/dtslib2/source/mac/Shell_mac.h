/*
**	Shell_mac.h		dtslib2
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

#ifndef Shell_mac_h
#define Shell_mac_h

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "Local_dts.h"
#include "Window_dts.h"


/*
**	DTSAppPriv class
*/
class DTSAppPriv
{
public:
	OSType			appCreator;		// app signature type / creator code
	int				appResFile;		// refnum of app's resource fork
	DTSWindow *		appFrontWin;	// front window
	
	// constructor/destructor
					DTSAppPriv();
					~DTSAppPriv();

private:
	// no copying
					DTSAppPriv( const DTSAppPriv & rhs );
	DTSAppPriv &	operator=( const DTSAppPriv & rhs );
};


/*
**	DTSTimer class
*/

// what's the basic Time type provided by the OS?
typedef CFAbsoluteTime				DTSTimer_t;		// floating point seconds since 1/1/2000
typedef DTSTimerPriv<DTSTimer_t>	DTSTimer;


//
//	Prefs helpers -- save typing when dealing with CFPreferences
//
namespace Prefs
{
	// the most general setter:
	// sets a preference named 'key' to 'value' (which must be a plist-valid type)
	void	Set( CFStringRef key, CFPropertyListRef value );
	
	// convenience overrides for common data types...
	void	Set( CFStringRef key, bool value );		// booleans -> CFBoolean
	void	Set( CFStringRef key, int value );		// integers -> CFNumber
	void	Set( CFStringRef key, const CGRect& value );	// CGRects -> a CFDictionary
	
	// the most general getter (returns NULL if 'key' not found).
	// Otherwise, don't forget to CFRelease the returned object!
	CFPropertyListRef	Copy( CFStringRef key );
	
	// convenience getters for specific common data types (just as above)
	// for these, you may optionally supply a 'defaultValue', which will be
	// returned if there's no [usable] already-saved preference with the given 'key'
	bool	GetBool( CFStringRef key, bool defaultValue = false );
	CFIndex	GetInt( CFStringRef key, CFIndex defaultValue = 0 );
	CGRect	GetRect( CFStringRef key, const CGRect& defaultValue = CGRectZero );
};


//
//	CFString helpers
//

/*
**	CreateCFString()
**	most common idiom for creating new CFStrings; from MacRoman C-string text
*/
static inline CFStringRef
CreateCFString( const char * txt, CFStringEncoding enc = kCFStringEncodingMacRoman )
{
	return CFStringCreateWithCString( kCFAllocatorDefault, txt, enc );
}

#if MAC_OS_X_VERSION_MAX_ALLOWED >= 1060
CFStringRef	CFStringCreateFormatted( CFStringRef fmt, ... ) CF_FORMAT_FUNCTION( 1, 2 );
#else
CFStringRef	CFStringCreateFormatted( CFStringRef fmt, ... ) /* __printflike( 1, 2 ) */;
#endif


/*
**	Internal Routines
*/
void		DoUpdate();
void		HandleEvent();
bool		ActivateWindows();
void		DoActivate( WindowRef macwin, bool activating );

uint		Mac2DTSModifiers( const EventRecord * event );
uint		Mac2DTSModifiers( UInt32 macMods );

#if TARGET_RT_MAC_MACHO
// handy test for weak-linked symbols
#  define IsCFMResolved( fn )	( &(fn) != nullptr )
#endif	// MachO


/*
**	Variables
*/
extern EventRecord	gEvent;

//extern DTSRect	gGrowLimit;
extern bool			gSuspended;
extern bool			gNetworking;

/*
**	Mac-specific environmental data
*/
struct OSFeatures
{								// examples values for 10.6.8 (SnowLeopard) vs 10.10.0 Yosemite
	SInt16	osVersionMajor;		// 10	10
	SInt16	osVersionMinor;		//	6	10
	SInt32	osVersionBugFix;	//	8	0
//	SInt32	appearanceVersion;
	
//	uint8_t	hasColorQD;
//	uint8_t	has85Controls;
//	uint8_t	has85Windows;
//	uint8_t	hasFloatingWindows;
//	uint8_t	hasDragMgr;
//	uint8_t	hasNavSvcs;
//	uint8_t	hasCarbonEvents;
//	uint8_t	hasThreadMgr;
	uint8_t	hasQuickTime;
//	uint8_t	hasAquaMenus;
//	uint8_t	hasHFSPlus;
};

extern OSFeatures	gOSFeatures;

//extern bool			(*gFreeProc)();

#endif  // Shell_mac_h

