/*
**	Controls_dts.h		dtslib2
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

#ifndef CONTROL_DTS_H
#define CONTROL_DTS_H

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "View_dts.h"


/*
**	DTSControl class
*/
class DTSControlPriv;
class DTSControl : public DTSView
{
public:
	DTSImplementNoCopy<DTSControlPriv> priv;
	
	// initialize by type -- one of the kControlXXX constants below
	DTSError	Init( int type, const char * title, DTSWindow * owner );
	DTSError	Init( int type, const char * title, DTSDlgView * owner );
	
	// set the range and the value
	void		SetRange( int min, int max );
	void		SetValue( int value );
	void		SetThumbSize( int value );	// only for scrollbars
//	void		DrawDefHilite() const;		// obsolete
//	void		SetDelay( int delay );		// don't need
//	void		SetSpeed( int speed );
	void		SetDeltaArrow( int delta );
	void		SetDeltaPage( int delta );
	
	int			GetMin() const;
	int			GetMax() const;
	int			GetValue() const;
//	int			GetThumbSize() const;		// do we need this?
//	int			GetDelay() const;			// don't need these two
//	int			GetSpeed() const;
	int			GetDeltaArrow() const;
	int			GetDeltaPage() const;
	
	// Hit() is called when the user clicks in the control.
	// It can be called multiple times if the user holds the mouse down long enough,
	// or for controls that do "live-tracking" (i.e. scrollbars & sliders)
	virtual void		Hit() {}
	
	// semi-private
	// overloaded DTSView routines
	virtual void		DoDraw();
	virtual void		Show();
	virtual void		Hide();
	virtual void		Move( DTSCoord h, DTSCoord v );
	virtual void		Size( DTSCoord width, DTSCoord height );
	
	// semi-private
	// overloaded DTSFocus routines
	virtual bool		Click( const DTSPoint& loc, ulong time, uint modifiers );
};

enum
{
	kControlHorzScrollBar = 0,
	kControlVertScrollBar,
	kControlPushButton,
	kControlCheckBox,
	kControlRadioButton,
	kControlHorzSlider,
	kControlVertSlider
	
	// TODO: support for...
	// - radio groups
	// - auto-toggling for checkboxes
	// - CFString titles
	// - compositing; standard-handler
};


#endif  // CONTROL_DTS_H
