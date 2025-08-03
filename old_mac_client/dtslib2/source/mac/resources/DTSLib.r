/*
**	DTSLib.r
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

// We want the new-style templates for these, with the auto-positioning data included.
#ifndef ALRT_RezTemplateVersion
#  define ALRT_RezTemplateVersion	1
#endif
#ifndef DLOG_RezTemplateVersion
#  define DLOG_RezTemplateVersion	1
#endif


//
//	System Includes
//
#include <Carbon/Carbon.r>


//
// Internal Definitions
//
enum
{
	// the base ID for most DTSLib resources
	rDTSLibBaseRsrcID			= 32000,

	// internal resource IDs for our dialogs and their dependents
	rGenericErrorAlert			= rDTSLibBaseRsrcID + 2,
	rGenericOKCancelAlert,
	rGenericYesNoCancelAlert,

	// other dialog IDs
	rColorSplash				= 32101,
	
	// Pict IDs for about boxen
	rAboutColorPictID			= 32100,	// yeah, this doesn't match rColorSplash
	rLicensePictID				= 32102,	// placeholder, mostly
	rCreditsPictID				= 32103,
};


//
// some resource type definitions.
//
type 'CSTR' {
	cstring;
};

type 'TMPL' {
	wide array Fields
		{
		pstring;
		literal longint;
		};
};


//
// and now the resources themselves!
//

// Dialog Item Lists

// about-box sizing constrains; let Rez do the arithmetic!
enum {
	kSpinLogoWidth	= 64,
	kSplashHeight	= 298,
	kSplashWidth	= 384,
	
	kLogoLeft		= 15,
	kLogoBotMargin	= 12,
	
	kAboutHeight	= 160,
	kAboutWidth		= kSplashWidth,
	
	kLicenseTop		= kAboutHeight + 4,
	kLicenseHeight	= 46,
	
	kCreditsHeight	= 80,
	kCreditsWidth	= 290,
	kCreditsTop		= kLicenseTop + kLicenseHeight + 4	
};


#if 0	// client and editor have local overrides of this
resource 'DITL' ( rColorSplash, "Color Splash", purgeable )
{
	{
	/* [1] */
	{kSplashHeight - kLogoBotMargin - kSpinLogoWidth, kLogoLeft,
	 kSplashHeight - kLogoBotMargin, kLogoLeft + kSpinLogoWidth},
	UserItem { disabled },
	
	/* [2] */
	{0, 0, kAboutHeight, kAboutWidth},
	Picture { disabled, rAboutColorPictID },
	
	/* [3] */
	{kLicenseTop, 0, kLicenseTop + kLicenseHeight, kSplashWidth},
	Picture { disabled, rLicensePictID },
	
	/* [4] */
	{kCreditsTop, kSplashWidth - kCreditsWidth, kCreditsTop + kCreditsHeight, kSplashWidth},
	Picture { disabled, rCreditsPictID }
	}
};


// Dialogs
resource 'DLOG' ( rColorSplash, "Color Splash", purgeable )
{
	{32, 64, 32 + kSplashHeight, 64 + kSplashWidth},
	dBoxProc,
	invisible,
	noGoAway,
	0x0,
	rColorSplash,
	"",
	alertPositionMainScreen
};
#endif  // 0


// Generic 'TEXT' file reference, for Finder, Standard File, etc.
resource 'FREF' ( rDTSLibBaseRsrcID, "Text", purgeable )
{
	'TEXT',	0,	""
};


// Generic folder file reference for GetFileType()
resource 'FREF' ( rDTSLibBaseRsrcID + 1, "Folder", purgeable )
{
	'fldr', 0, ""
};


#if 0	// both client and editor have local overrides of this
// Appearance extensions for Dialogs
resource 'dlgx' ( rColorSplash, "color splash", purgeable )
{
	versionZero {
		kDialogFlagsUseThemeBackground	| 
		kDialogFlagsUseControlHierarchy	|
		kDialogFlagsHandleMovableModal	|
		kDialogFlagsUseThemeControls
	}
};
#endif  // 0


// generic list-control definition, for DTSDlgLists
resource 'ldes' ( 128, "generic", purgeable )
{
	versionZero {
		0,					// rows at inception
		1,					// columns
		18,					// cell height
		0,					// cell width (0 means 'auto')
		hasVertScroll,		// scroll bars wanted?
		noHorizScroll,
		0,					// LDEF ID to use
		noGrowSpace			// save room for window grow icon?
	}
};


#if 0	// replaced by CG drawing

// Pictures for the about box rotating logo
// (which share much common data)
// include "DTSLib.rsrc" 'PICT';

#define LOGO_PICT_HEADER  \
	$"0096 0000 0000 0080 0080 0011 02FF 0C00"	\
	$"FFFF FFFF 0000 0000 0000 0000 0080 0000"	\
	$"0080 0000 0000 0000 0001 000A 0000 0000"	\
	$"0080 0080 0032 0000 0000 0080 0080 0061"	\
	$"0002 0002 007E 007E"

#define LOGO_PICT_TRAILER \
	$"0008 000A 0071 001A 0002 000A 005F 0076"	\
	$"0002 0040 005F 000A 005F 0076 0002 0040"	\
	$"0007 0002 0002 0008 0008 0050 0000 0000"	\
	$"0080 0080 00FF"

data 'PICT' ( 32000, purgeable )
{
	LOGO_PICT_HEADER
	$"005A 00B4 0051 0021"
	$"0002 005F 0040 0052 0021 0040 005F 007E"
	LOGO_PICT_TRAILER
};

data 'PICT' ( 32001, purgeable )
{
	LOGO_PICT_HEADER
	$"0078 00B4 0051 0012"
	$"0006 0050 0044 0052 0030 003C 006E 007A"
	LOGO_PICT_TRAILER
};

data 'PICT' ( 32002, purgeable )
{
	LOGO_PICT_HEADER
	$"0096 00B4 0051 0006"
	$"0011 0044 004F 0052 003C 0031 007A 006F"
	LOGO_PICT_TRAILER
};

data 'PICT' ( 32003, purgeable )
{
	LOGO_PICT_HEADER
	$"00B4 00B4 0051 0002"
	$"0021 0040 005F 0052 0040 0021 007E 005F"
	LOGO_PICT_TRAILER
};

data 'PICT' ( 32004, purgeable )
{
	LOGO_PICT_HEADER
	$"00D2 00B4 0051 0006"
	$"0030 0044 006E 0052 003C 0012 007A 0050"
	LOGO_PICT_TRAILER
};

data 'PICT' ( 32005, purgeable )
{
	LOGO_PICT_HEADER
	$"00F0 00B4 0051 0012"
	$"003C 0050 007A 0052 0030 0006 006E 0044"
	LOGO_PICT_TRAILER
};

data 'PICT' ( 32006, purgeable )
{
	LOGO_PICT_HEADER
	$"010E 00B4 0051 0021"
	$"0040 005F 007E 0052 0021 0002 005F 0040"
	LOGO_PICT_TRAILER
};

data 'PICT' ( 32007, purgeable )
{
	LOGO_PICT_HEADER
	$"012C 00B4 0051 0030"
	$"003C 006E 007A 0052 0012 0006 0050 0044"
	LOGO_PICT_TRAILER
};

data 'PICT' ( 32008, purgeable )
{
	LOGO_PICT_HEADER
	$"014A 00B4 0051 003C"
	$"0030 007A 006E 0052 0006 0012 0044 0050"
	LOGO_PICT_TRAILER
};

data 'PICT' ( 32009, purgeable )
{
	LOGO_PICT_HEADER
	$"0168 00B4 0051 0040"
	$"0021 007E 005F 0052 0002 0021 0040 005F"
	LOGO_PICT_TRAILER
};

data 'PICT' ( 32010, purgeable )
{
	LOGO_PICT_HEADER
	$"0186 00B4 0051 003C"
	$"0011 007A 004F 0052 0006 0031 0044 006F"
	LOGO_PICT_TRAILER
};

data 'PICT' ( 32011, purgeable )
{
	LOGO_PICT_HEADER
	$"01A4 00B4 0051 0030"
	$"0006 006E 0044 0052 0012 003C 0050 007A"
	LOGO_PICT_TRAILER
};

#endif  // 0

