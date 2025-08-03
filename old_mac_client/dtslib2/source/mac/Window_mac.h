/*
**	Window_mac.h		dtslib2
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

#ifndef Window_mac_h
#define Window_mac_h

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "Local_dts.h"
#include "Window_dts.h"


/*
**	class DTSWindowPriv
*/
class DTSWindowPriv
{
public:
	WindowPtr	winWindow;
//	DTSRect		winZoomMargin;
//	DTSRect		winUnzoomRect;
	DTSCoord	winZoomWidth;
	DTSCoord	winZoomHeight;
//	bool		winHasGrowIcon;
	bool		winSaveVisible;
	bool		winIsFloater;
	bool		winGetFrontClick;
	
					// constructor/destructor
					DTSWindowPriv();
					~DTSWindowPriv();
	
private:
					// no copies
					DTSWindowPriv( const DTSWindowPriv& );
	DTSWindowPriv&	operator=( const DTSWindowPriv& );
};


enum
{
	kUnknownWinKind = kApplicationWindowKind,
	kLayerWinKind,		// not used
	kDocWinKind,
	kFloaterWinKind
};

// additional Mac window-kind flags, extending kWKindCloseBox et al.
enum
{
	kWKindLiveResize		= 0x02000000,
	kWKindStandardHandler	= 0x01000000,
	kWKindCompositing		= 0x00800000,
	kWKindHighResolution	= 0x00400000,
};


/*
**	Routines
*/
//void	DTSWindow_Hilite( DTSWindow& win );	// no-op
void	DTSWindow_MouseDown( DTSWindow& win, const DTSPoint& where, ulong when, uint mods );
void	DTSWindow_Drag( DTSWindow& win, const DTSPoint& where, bool dontComeFront );
void	DTSWindow_Grow( DTSWindow& win, const DTSPoint& where );
void	DTSWindow_GoAway( DTSWindow& win, const DTSPoint& where );
void	DTSWindow_Zoom( DTSWindow& win, const DTSPoint& where, int partcode );
void	DTSWindow_Update( DTSWindow& win );


/*
**	Conversions from DTS types to Macintosh types
*/
inline WindowRef
DTS2Mac( const DTSWindow * w )
{
	return ( w && w->priv.p ) ? w->priv.p->winWindow : nullptr;
}

// and vice-versa
DTSWindow *		FindDTSWindow( WindowRef );

#endif  // Window_mac_h

