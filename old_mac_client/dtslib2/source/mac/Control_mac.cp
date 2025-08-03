/*
**	Control_mac.cp		dtslib2
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

#include "Control_dts.h"
#include "Dialog_dts.h"
#include "Memory_dts.h"
#include "Shell_dts.h"
#include "Window_dts.h"
#include "Control_mac.h"
#include "Dialog_mac.h"
#include "Shell_mac.h"
#include "Window_mac.h"
#include "View_mac.h"


/*
**	Entry Routines
**
**	DTSControl
*/


/*
**	Internal Routines
*/
static void 	ActionProc( HIViewRef, ControlPartCode );


/*
**	Internal Variables
*/
static int		gOldValue;

/*
**	DTSControl
*/
DTSDefineImplementFirmNoCopy(DTSControlPriv)


/*
**	DTSControlPriv::DTSControlPriv()
*/
DTSControlPriv::DTSControlPriv() :
	controlHdl( nullptr ),
	controlDeltaArrow( 1 ),
	controlDeltaPage( 2 )
{
}


/*
**	DTSControlPriv::~DTSControlPriv()
*/
DTSControlPriv::~DTSControlPriv()
{
	if ( controlHdl )
		DisposeControl( controlHdl );
}


/*
**	DTSControl::Init()
*/
DTSError
DTSControl::Init( int ctype, const char * title, DTSWindow * owner )
{
	DTSControlPriv * p = priv.p;
	if ( not p )
		return -1;
	
	if ( not owner )
		{
		if ( not gDTSApp )
			{
#ifdef DEBUG_VERSION
			VDebugStr( "Application must have a DTSApp." );
#endif
			return -1;
			}
		owner = gDTSApp->GetFrontWindow();
		if ( not owner )
			{
#ifdef DEBUG_VERSION
			VDebugStr( "DTSControl must have an owner window." );
#endif
			return -1;
			}
		}
	
	DTSError result = p->InitPriv( ctype, title, DTS2Mac(owner), this );
	Attach( owner );
	return result;
}


/*
**	DTSControl::Init()
*/
DTSError
DTSControl::Init( int ctype, const char * title, DTSDlgView * owner )
{
	DTSControlPriv * p = priv.p;
	if ( not p )
		return -1;
	
	if ( not owner )
		{
#ifdef DEBUG_VERSION
		VDebugStr( "DTSControl must have an owner dialog." );
#endif
		return -1;
		}
	
	return p->InitPriv( ctype, title, GetDialogWindow(DTS2Mac(owner)), this );
}


/*
**	DTSControlPriv::InitPriv()
*/
DTSError
DTSControlPriv::InitPriv( int ctype, const char * title, WindowRef macWindow, DTSControl * dctl )
{
//	controlType       = ctype;
//	controlDelay      = 0;
//	controlSpeed      = 0;
//	controlDeltaArrow = 1;
//	controlDeltaPage  = 2;
	
	CFStringRef cstitle = CreateCFString( title );
	__Check( cstitle );
	if ( not cstitle )
		return memFullErr;
	
	const Rect bounds = { 0, 0, 16, 16 };
	
	// one-time set-up for the UPPs
#if TARGET_RT_MAC_MACHO
	const ControlActionUPP actionUPP = ActionProc;
#else
	static ControlActionUPP	actionUPP;
	if ( not actionUPP )
		{
		actionUPP = NewControlActionUPP( ActionProc );
		__Check( actionUPP );
		}
#endif  // TARGET_RT_MAC_MACHO
	
	OSStatus err = noErr;
	HIViewRef ctl = nullptr;
	switch ( ctype )
		{
		case kControlHorzScrollBar:
		case kControlVertScrollBar:
			err = CreateScrollBarControl( macWindow, &bounds,
					0, 0, 0,	// cur value, min, max
					0,			// view-size
					true,		// want live tracking
					actionUPP, &ctl );
			break;
		
		case kControlHorzSlider:
		case kControlVertSlider:
			err = CreateSliderControl( macWindow, &bounds,
					0, 0, 0,	// cur value, min, max
					kControlSliderDoesNotPoint,		// non-directional indicator widget
					0,			// no tick marks
					true,		// want live tracking
					actionUPP, &ctl );
			break;
		
		case kControlPushButton:
			err = CreatePushButtonControl( macWindow, &bounds, cstitle, &ctl );
			break;
		
		case kControlCheckBox:
			err = CreateCheckBoxControl( macWindow, &bounds, cstitle,
					0,		// curr value
					false,	// does not autotoggle
							// (but our code would be oh-so-much simpler if it did...)
					&ctl );
			break;
		
		case kControlRadioButton:
			err = CreateRadioButtonControl( macWindow, &bounds, cstitle,
					0, false, &ctl );	// like checkbox
			break;
		
		default:
			err = paramErr;
#ifdef DEBUG_VERSION
			VDebugStr( "Invalid type passed to DTSControl." );
#endif
		}
	CFRelease( cstitle );
	__Check_noErr( err );
	
	if ( noErr == err )
		{
		if ( DTSViewPriv * vp = dctl->DTSView::priv.p )
			{
			* DTS2Mac( &vp->viewBounds ) = bounds;
#ifndef AVOID_QUICKDRAW
			vp->viewContent.Set( &vp->viewBounds );
#endif
			vp->viewVisible = true;
			}
		
		// enable reverse-lookup from HIViewRef to DTSControl*
		__Verify_noErr( SetControlProperty( ctl,
			kDTSControlOwnerCreator, kDTSControlOwnerTag, sizeof dctl, &dctl ) );
		}
	
	controlHdl = ctl;
	return err;
}


/*
**	DTSControl::SetRange()
*/
void
DTSControl::SetRange( int min, int max )
{
	DTSControlPriv * p = priv.p;
	if ( not p )
		return;
	HIViewRef ctl = p->controlHdl;
	
//	assert( min <= max );
	int value = HIViewGetValue( ctl );
	if ( value < min )
		value = min;
	else
	if ( value > max )
		value = max;
	
	__Verify_noErr( HIViewSetMinimum( ctl, min ) );
	__Verify_noErr( HIViewSetMaximum( ctl, max ) );
	__Verify_noErr( HIViewSetValue(   ctl, value ) );
}


/*
**	DTSControl::SetValue()
*/
void
DTSControl::SetValue( int value )
{
	if ( const DTSControlPriv * p = priv.p )
		__Verify_noErr( HIViewSetValue( p->controlHdl, value ) );
}


/*
**	DTSControl::SetThumbSize()
*/
void
DTSControl::SetThumbSize( int value )
{
	if ( const DTSControlPriv * p = priv.p )
		__Verify_noErr( HIViewSetViewSize( p->controlHdl, value ) );
}


#if 0	// not needed
/*
**	DTSControl::SetDelay()
*/
void
DTSControl::SetDelay( int delay )
{
	if ( DTSControlPriv * p = priv.p )
		p->controlDelay = delay;
}


/*
**	DTSControl::SetSpeed()
*/
void
DTSControl::SetSpeed( int speed )
{
	if ( DTSControlPriv * p = priv.p )
		p->controlSpeed = speed;
}
#endif  // 0


/*
**	DTSControl::SetDeltaArrow()
*/
void
DTSControl::SetDeltaArrow( int delta )
{
	if ( DTSControlPriv * p = priv.p )
		p->controlDeltaArrow = delta;
}


/*
**	DTSControl::SetDeltaPage()
*/
void
DTSControl::SetDeltaPage( int delta )
{
	if ( DTSControlPriv * p = priv.p )
		p->controlDeltaPage = delta;
}


/*
**	DTSControl::GetMin()
*/
int
DTSControl::GetMin() const
{
	if ( const DTSControlPriv * p = priv.p )
		return HIViewGetMinimum( p->controlHdl );
	
	return 0;
}


/*
**	DTSControl::GetMax()
*/
int
DTSControl::GetMax() const
{
	if ( const DTSControlPriv * p = priv.p )
		return HIViewGetMaximum( p->controlHdl );
	
	return 0;
}


/*
**	DTSControl::GetValue()
*/
int
DTSControl::GetValue() const
{
	if ( const DTSControlPriv * p = priv.p )
		return HIViewGetValue( p->controlHdl );
	
	return 0;
}


#if 0	// not needed
/*
**	DTSControl::GetDelay()
*/
int
DTSControl::GetDelay() const
{
	const DTSControlPriv * p = priv.p;
	return p ? p->controlDelay : 0;
}


/*
**	DTSControl::GetSpeed()
*/
int
DTSControl::GetSpeed() const
{
	const DTSControlPriv * p = priv.p;
	return p ? p->controlSpeed : 0;
}
#endif  // 0


/*
**	DTSControl::GetDeltaArrow()
*/
int
DTSControl::GetDeltaArrow() const
{
	const DTSControlPriv * p = priv.p;
	return p ? p->controlDeltaArrow : 0;
}


/*
**	DTSControl::GetDeltaPage()
*/
int
DTSControl::GetDeltaPage() const
{
	const DTSControlPriv * p = priv.p;
	return p ? p->controlDeltaPage : 0;
}


/*
**	DTSControl::DoDraw()
*/
void
DTSControl::DoDraw()
{
	if ( DTSControlPriv * p = priv.p )
		{
		Draw1Control( p->controlHdl );
//		what if it's a compositing window?
		}
}


/*
**	ShowHideControl()
**	hide or reveal
*/
static inline void
ShowHideControl( const DTSControl * ctl, bool bShow )
{
	if ( DTSControlPriv * p = ctl->priv.p )
		{
		__Verify_noErr( HIViewSetVisible( p->controlHdl, bShow ) );
		}
}


/*
**	DTSControl::Show()
*/
void
DTSControl::Show()
{
	DTSView::Show();
	ShowHideControl( this, true );
}


/*
**	DTSControl::Hide()
*/
void
DTSControl::Hide()
{
	DTSView::Hide();
	ShowHideControl( this, false );
}


/*
**	DTSControl::Move()
*/
void
DTSControl::Move( DTSCoord h, DTSCoord v )
{
	DTSView::Move( h, v );
	
	DTSControlPriv * p = priv.p;
	if ( not p )
		return;
	
	HIViewRef ctl = p->controlHdl;
	
#if 0
	// older way
	Rect b;
	GetControlBounds( ctl, &b );
	OffsetRect( &b, h - b.left, v - b.top );
	SetControlBounds( ctl, &b );
#else
	// I tried to use HIViewPlaceInSuperviewAt() here instead, but couldn't get it to work.
	HIRect b;
	OSStatus err = HIViewGetFrame( ctl, &b );
	__Check_noErr( err );
	if ( noErr ==  err )
		{
		b.origin.x = h;
		b.origin.y = v;
		__Verify_noErr( HIViewSetFrame( ctl, &b ) );
		}
#endif  // 0
}


/*
**	DTSControl::Size()
*/
void
DTSControl::Size( DTSCoord width, DTSCoord height )
{
	DTSView::Size( width, height );
	
	DTSControlPriv * p = priv.p;
	if ( not p )
		return;
	HIViewRef ctl = p->controlHdl;
	
#if 0
	Rect b;
	GetControlBounds( ctl, &b );
	b.right = b.left + width;
	b.bottom = b.top + height;
	SetControlBounds( ctl, &b );
#else
	HIRect frame;
	OSStatus err = HIViewGetFrame( ctl, &frame );
	__Check_noErr( err );
	if ( noErr == err )
		{
		frame.size.width = width;
		frame.size.height = height;
		__Verify_noErr( HIViewSetFrame( ctl, &frame ) );
		}
#endif  // 0
}


/*
**	DTSControl::Click()
*/
bool
DTSControl::Click( const DTSPoint& loc, ulong /* time */, uint dtsmods )
{
	DTSControlPriv * p = priv.p;
	if ( not p )
		return false;
	
	Point hitLoc = * DTS2Mac( &loc );
	HIViewRef macControl = p->controlHdl;
	
	// for compositing windows, TestControl() needs coordinates local to the control itself,
	// not to the window's content region
	if ( WindowRef win = GetControlOwner( macControl ) )
		{
#if MAC_OS_X_VERSION_MIN_REQUIRED >= 1050
		if ( HIWindowTestAttribute( win, kHIWindowBitCompositing ) )
#else
		WindowAttributes attrs = 0;
		if ( noErr == GetWindowAttributes( win, &attrs )
		&&   (kWindowCompositingAttribute & attrs) )
#endif
			{
			DTSRect bounds;
			GetBounds( &bounds );
			
			// offset hitLoc to view-relative coords
			hitLoc.h -= bounds.rectLeft;
			hitLoc.v -= bounds.rectTop;
			}
		}
	
	// what part of the control is being tickled?
	// Bail early if it's not ticklish there.  (Can that even happen?)
	int startPart = TestControl( macControl, hitLoc );
	if ( kControlNoPart == startPart )
		return false;
	
	// see jeremiad below about globals
	gOldValue = HIViewGetValue( macControl );
	
//	StDrawEnv saveEnv( this );
	
	// reverse-engineer the DTS modifiers into Mac ones (sigh)
	EventModifiers macMods = 0;
	if ( dtsmods & kKeyModMenu )
		macMods |= cmdKey;
	if ( dtsmods & kKeyModShift )
		macMods |= shiftKey;
	if ( dtsmods & kKeyModOption )
		macMods |= optionKey;
	if ( dtsmods & kKeyModControl )
		macMods |= controlKey;
	
	// ask toolbox to track the control.
	// If it was a live-tracking variant, our ActionProc() will get called
	// repeatedly until the tracking ends.
	ControlPartCode whatHit = HandleControlClick( macControl, hitLoc,
								macMods, ControlActionUPP( -1 ) );
	
	// inform the control that it's been poked (unless the click was aborted)
	if ( whatHit != kControlNoPart )
		Hit();
	
	return true;
}


/*
**	ControlActionProc()
**	used for scrollbars & sliders; called repeatedly during the tracking session
*/
void
ActionProc( HIViewRef macControl, ControlPartCode part )
{
	// get the DTS version of the control
	DTSControl * dtsControl = nullptr;
	__Verify_noErr( GetControlProperty( macControl,
					kDTSControlOwnerCreator, kDTSControlOwnerTag,
					sizeof dtsControl, nullptr, &dtsControl ) );
	if ( not dtsControl )
		return;
	
	// and its private sidecar data
	DTSControlPriv * p = dtsControl->priv.p;
	if ( not p )
		return;
	
	// paranoia
	__Check( macControl == p->controlHdl );
	
	// decide how to update the value of the Mac control
	int value = HIViewGetValue( macControl );
	int delta = 0;
	
	switch ( part )
		{
		case kControlIndicatorPart:
			// no change; we're being live-scrolled
			break;
		
		case kControlPageUpPart:
			delta = - p->controlDeltaPage;
			break;
		case kControlPageDownPart:
			delta = p->controlDeltaPage;
			break;
		
		case kControlUpButtonPart:
			delta = - p->controlDeltaArrow;
			break;
		
		case kControlDownButtonPart:
			delta = p->controlDeltaArrow;
			break;
		
		default:
			// no change: it's a pushbutton or checkbox or radio or some other weird kind of part
			break;
		}
	
	// actually update the value, if there's a change
	if ( delta )
		__Verify_noErr( HIViewSetValue( macControl, value + delta ) );
	
	// call the hit procedure whenever the value changes.
	//
	// I hate that we're using a global for this. The alternative would be to
	// call the Hit() routine unconditionally, and require each DTSControl subclass
	// (or its controller, whatever it might be: e.g. a window object) to maintain
	// its own internal notion of "previous value"; it would then have to ignore hits
	// that leave the value unchanged.
	// In principle that shouldn't be hard; just tedious tracking them all down.
	
	if ( value + delta != gOldValue )
		dtsControl->Hit();
	
	gOldValue = value;
}


