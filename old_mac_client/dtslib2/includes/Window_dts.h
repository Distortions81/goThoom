/*
**	Window_dts.h		dtslib2
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

#ifndef Windows_dts_h
#define Windows_dts_h

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "Local_dts.h"
#include "Platform_dts.h"
#include "View_dts.h"


/*
**	DTSWindow class
*/
class DTSWindowPriv;
class DTSWindow : public DTSView
{
public:
	DTSImplementNoCopy<DTSWindowPriv> priv;
	
	// constructor/destructor
	virtual			~DTSWindow();
	
	// initialize the window
	DTSError		Init( uint kind );
	
	// set the window's title
	void			SetTitle( const char * title );
	
	// invalidate portions of the window to force an update
	void			Invalidate();
	void			Invalidate( const DTSRect * rect );
#ifndef DTS_XCODE
	void			Invalidate( const DTSRegion * rgn );
#endif
	
	// get the location of the mouse relative to this window
	void			GetMouseLoc( DTSPoint * oMouseLoc ) const;
	
	// get the location of the window's top-left corner (content not structure)
	void			GetPosition( DTSPoint * oGlobalPosition ) const;
	
	// return true if the window can be closed
	virtual bool	CanClose();
	
	// close the window (presumably CanClose() returned true)
	virtual void	Close();
	
	// activate and deactivate the window
	virtual void	Activate();
	virtual void	Deactivate();
	
	// handle a menu command (for convenience)
	virtual bool	DoCmd( long menuItem );
	
	// move the window around in z-order
	void			GoToFront();
	void			GoToBack();
	void			GoBehind( const DTSWindow * behind );
	
	// set the zoom size
	void			SetZoomSize( DTSCoord width, DTSCoord height );
	
	// get & set minimize/collapse state
	bool			IsMinimized() const;
	DTSError		SetMinimized( bool minimize );
	
	// semi-private
	// overloaded DTSView routines
	virtual void	Show();
	virtual void	Hide();
	virtual void	Move( DTSCoord left, DTSCoord top );
	virtual void	Size( DTSCoord width, DTSCoord height );
};

enum
{
	kWKindDocument			= 0,
	kWKindDlgBox			= 1,
	kWKindPlainDlgBox		= 2,
	kWKindMovableDlgBox		= 3
};
	
enum
{
	kWKindCloseBox			= 0x80000000,
	kWKindZoomBox			= 0x40000000,
	kWKindGrowBox			= 0x20000000,
	kWKindFloater			= 0x10000000,
	kWKindGetFrontClick		= 0x04000000
};


#endif  // Windows_dts_h
