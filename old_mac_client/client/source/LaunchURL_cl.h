/*
**	LaunchURL_cl.h		ClanLord
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

#ifndef LAUNCHURL_CL_H
#define LAUNCHURL_CL_H

#include "StyledTextField_cl.h"

const int kLaunchURLHandler_PointCursorID		= 200;

// Characters in this array count as spaces
#define kLaunchURLHandler_Breaks " \t\n\r><\"()[],"

const int kLaunchURLHandler_HelperNameLength	= 16;

#ifdef ARINDAL
# define kClanLordURL	"http://www.arindal.com"
# define kTroubleURL	"http://www.arindal.com/trouble.html"
# define kDataURL		"http://www.arindal.com/downloads/data/"
#else
# define kClanLordURL	"https://www.deltatao.com/clanlord/"
# define kTroubleURL	"https://www.deltatao.com/clanlord/trouble.html"
# define kDataURL		"https://www.deltatao.com/downloads/clanlord/data/"
#endif // ARINDAL

// Error Codes
enum
{
	kLaunchHandlerError_ZeroSelection =	-211,
	kLaunchHandlerError_NoURLFound,
	kLaunchHandlerError_ICNotAvailable
};

// Index for array of helpers we care about
enum
{
	// 'https' must come before 'http' otherwise it'll never be recognized separately
	kLaunchHandler_HTTPSHelper,
	kLaunchHandler_HTTPHelper,
	kLaunchHandler_MAILTOHelper,
	kLaunchHandler_FTPHelper,
	kLaunchHandler_NumHelpers
};

// The styles that LaunchURL sets up
enum
{
	kLaunchURL_kLinkStyle,
	kLaunchURL_kDisabledLinkStyle,
	kLaunchURL_NumStyles
};

// Info to sort of generalize the types of helpers
struct LaunchHandlerHelperInfo
{
	char	scheme[ kLaunchURLHandler_HelperNameLength ];
	bool	available;
};


// A global class that an app can initialize and use to launch URLs
class LaunchURLHandler
{
private:
	LaunchHandlerHelperInfo	helpers[ kLaunchHandler_NumHelpers ];
	
public:
	// ctor dtor defaulted
	
	OSStatus	Init();
//	bool		Available() const { return true; }
	OSStatus	LaunchURL( const char * text,long * urlStart, long * urlEnd );
	OSStatus	ParseForURL( const char * data, long * selStart, long * selEnd ) const;
};


// A class which implements links in a text field
class URLStyledTextField : public StyledTextField
{
public:
	bool			IsLink( const DTSPoint& loc ) const;
	void			StyleTextURL( bool enabled );
	void			StyleInsert( const char * text, const CLStyleRecord * style );
	
	virtual bool	Click( const DTSPoint& loc, ulong time, uint modifiers );
	virtual void 	Idle();
};


// A class for a menu which launches URLs stored in resources from the Help menu	
class URLMenu
{
public:
	bool		DoCmd( int menuCmd );
};


extern	LaunchURLHandler	gLaunchURLHandler;
extern	URLMenu				gURLMenu;

#endif  // LAUNCHURL_CL_H

