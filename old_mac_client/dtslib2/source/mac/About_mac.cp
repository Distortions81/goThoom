/*
**	About_mac.cp		dtslib2
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

#include "About_dts.h"


/*
**	Entry Routines
**
**	ShowAboutBox();
**	SplashAboutBox();
**	HideAboutBox();
*/

/*
**	Definitions
*/

// to avoid 64-to-32 warnings
#define F_PI	float(M_PI)

enum
{
	// about-box dialog item indices
	abSpinLogo = 1,
};

// How many frames in the logo animation?  More is better, but uses additional CPU.
#define	kNumLogoSteps			(48)

// How long it takes for the spinning yin-yang-thang to complete one rotation (in seconds)
#define	kAboutTimePerRev		(2 * kEventDurationSecond)

// How long to wait before starting the animation? (Classic DTSLib used 5 seconds)
#define	kAboutStartDelay		(2 * kEventDurationSecond)

// How long between each animation frame? (inverse of frame rate)
#define	kAboutAnimDelay			(kAboutTimePerRev / kNumLogoSteps)

// ratio of yy's rotation rate to triangle's rate
#define	kCounterRotateRate		(4)


/*
**	Internal Routines
*/
static void		InitSpinLogo( DialogRef, int logoItem );
static OSStatus	AB_HandleEvent( EventHandlerCallRef, EventRef, void * );


/*
**	Internal Variables
*/
static DialogRef	gSplashDlg;
static EventLoopTimerRef	gLogoTimer;
static int			gLogoIndex;
static uint			gAboutBoxFlags;


/*
**	GetSplashWindow()
*/
static inline WindowRef
GetSplashWindow()
{
	return gSplashDlg ? GetDialogWindow( gSplashDlg ) : nullptr;
}


/*
**	ShowAboutBox()
**
**	display about box, modally, until dismissed
*/
void
ShowAboutBox( uint flags )
{
	if ( not gSplashDlg )
		{
		SplashAboutBox( flags );
		
		if ( WindowRef w = GetSplashWindow() )
			{
			const EventTypeSpec evts[] = {
				{ kEventClassMouse, kEventMouseDown },
				{ kEventClassKeyboard, kEventRawKeyDown }
			};
			InstallWindowEventHandler( w, AB_HandleEvent,
					GetEventTypeCount( evts ), evts, nullptr, nullptr );
			
			// start the spinner if there is one
			if ( gLogoTimer )
				__Verify_noErr( SetEventLoopTimerNextFireTime( gLogoTimer,
									kAboutStartDelay ) );
			
			// let system handle everything until we're dismissed
			RunAppModalLoopForWindow( w );
			
			HideAboutBox();
			}
		}
}


/*
**	SplashAboutBox()
**
**	load & display the dialog, but don't process any events
*/
void
SplashAboutBox( uint flags )
{
	// bail if already showing
	if ( gSplashDlg )
		return;
	
	// save the flags
	gAboutBoxFlags = flags;
	
	// load the dialog and the spinny thingy
	DialogRef dlg = GetNewDialog( rAboutColorDlgID, nullptr, kFirstWindowOfClass );
	gSplashDlg = dlg;
	CheckPointer( dlg );
	
	InitSpinLogo( dlg, abSpinLogo );
	
	// reveal it
	ShowWindow( GetDialogWindow( dlg ) );
}


/*
**	HideAboutBox()
*/
void
HideAboutBox()
{
	// cancel the redraw timer
	if ( gLogoTimer )
		{
		__Verify_noErr( SetEventLoopTimerNextFireTime( gLogoTimer, kEventDurationForever ) );
		__Verify_noErr( RemoveEventLoopTimer( gLogoTimer ) );
		gLogoTimer = nullptr;
		}
	
	// tear down the dialog
	if ( gSplashDlg )
		{
		DisposeDialog( gSplashDlg );
		gSplashDlg = nullptr;
		}
}


/*
**	AboutTimerProc()
**
**	callback from the redraw timer
*/
static void
AboutTimerProc( EventLoopTimerRef /* timer */, void * ud )
{
	// update the animation index
	enum { kMaxIndex = kNumLogoSteps * kCounterRotateRate };
	gLogoIndex = (gLogoIndex + 1) % kMaxIndex;
	
	// force a redraw of the user-item
	HIViewRef userItem = static_cast<HIViewRef>( ud );
	__Check( userItem );
	if ( userItem )
		__Verify_noErr( HIViewSetNeedsDisplay( userItem, true ) );
}


/*
**	InitSpinLogo()
*/
void
InitSpinLogo( DialogRef dlg, int item )
{
	// bail if spinny not wanted
	if ( gAboutBoxFlags & kDontDoSpinningLogo )
		return;
	
	gLogoIndex = -1;
	
	// install the redraw function and timer.
	HIViewRef ctl( nullptr );
	OSStatus err = GetDialogItemAsControl( dlg, item, &ctl );
	__Check_noErr( err );
	__Check( ctl );
	if ( noErr == err && ctl )
		{
		const EventTypeSpec evt = { kEventClassControl, kEventControlDraw };
		__Verify_noErr( InstallControlEventHandler( ctl, AB_HandleEvent,
							1, &evt, ctl, nullptr ) );
		
		// set up the timer, but in a suspended state;
		// it won't actually start running until ShowAboutBox()
		__Verify_noErr( InstallEventLoopTimer( GetCurrentEventLoop(),
							kEventDurationForever, // kAboutStartDelay,
							kAboutAnimDelay,
							AboutTimerProc,
							ctl,
							&gLogoTimer ) );
		}
}


//
//	parameters for drawing the logo
//

// the logo is based on a notional coordinate system that's LOGOBOX units tall/wide
#define LOGOBOX			(1.0F)
#define HALFBOX			(LOGOBOX * 0.5F)		// half of LOGOBOX
#define RADIUS			(LOGOBOX * 0.495F)		// a tiny bit less than HALFBOX
#define HalfRad			(RADIUS  * 0.5F)		// radii of the inner semicircular curves
#define kOuterStroke	(LOGOBOX / 120)			// a hairline, or maybe 2 hairs

#define kCosine			(RADIUS * 0.8660254F)	// cos(30)
#define kSine			(RADIUS * 0.5F)			// sin(30)

// intensities and alpha of the RGB colors, for the non-B&W version
#define kYY_Primary		(0.9F)	// "yin"'s  RGB is { 0, sec, prim } -- blueish
#define	kYY_Secondary	(0.2F)	// "yang"'s RGB is { 0, prim, sec } -- greenish
#define kYY_Range		(kYY_Primary - kYY_Secondary)	// range between primary & secondary
#define kYY_Alpha		(1)		// colored version's alpha might (some day) differ from BW


/*
**	SetLogoTrianglePath()
**
**	appends the apexes (or apices) of the central triangle to the current path.
**	'clockwise' controls the "winding direction" of the added path segments.
**	also closes the path.
*/
static void
SetLogoTrianglePath( CGContextRef c, bool clockwise )
{
	// OS 10.7 Lion seems to have a CG bug with winding directions
	if ( 10 == gOSFeatures.osVersionMajor && 7 == gOSFeatures.osVersionMinor )
		clockwise = not clockwise;
	
	CGContextMoveToPoint( c, 0, -RADIUS );
	if ( clockwise )
		{
		CGContextAddLineToPoint( c,  kCosine, kSine );
		CGContextAddLineToPoint( c, -kCosine, kSine );
		}
	else
		{
		CGContextAddLineToPoint( c, -kCosine, kSine );
		CGContextAddLineToPoint( c,  kCosine, kSine );
		}
	
	CGContextClosePath( c );
}


/*
**	DrawYinYang()
**
**	draw the curvy bits of the logo
**	(or rather, half of it: this routine actually gets called (at least) twice per frame,
**	with the CTM rotated 180 degrees after each call. In other words, one call
**	produces the "yin" and the other makes the "yang".)
**	Each half-logo is painted using whatever fill color was last set by our caller.
**	The colored codepath calls this four times per frame, once for each combination of...
**	  -	green vs blue fill color
**	  - clipping inside vs outside the triangle
*/
static void
DrawYinYang( CGContextRef c, bool invert )
{
	// handy mnemonics for CGContextAddArc()'s "clockwise" argument
	enum {
		kCCW = 0,		// counter-clockwise
		kCW = 1
	};
	
	// determine the current angle, based on number of steps
	CGFloat angle = gLogoIndex * 2 * F_PI / kNumLogoSteps;
	if ( invert )
		angle += F_PI;	// i.e. 180 degrees more
	// (at the end of the day, 'angle' can end up being much higher than 2*pi. But that's OK.)
	
	CGContextRotateCTM( c, angle );
	
	// draw the "positive" lower half-circle
	CGContextBeginPath( c );
	CGContextMoveToPoint( c, RADIUS, 0 );
	CGContextAddArc( c, 0, 0, RADIUS, 0, F_PI, kCW );
	
	// exclude the negative small right-hand circle scallop
	CGContextAddArc( c, -HalfRad, 0, HalfRad, F_PI, 0, kCCW );
	
	// include the positive small left-hand circle bump
	CGContextAddArc( c, HalfRad, 0, HalfRad, F_PI, 0, kCW );
	
	// paint it
	CGContextFillPath( c );
	
	// undo the rotation
	CGContextRotateCTM( c, -angle );
}


/*
**	DrawLogoCG()
**
**	draw one frame of the complete logo
*/
static void
DrawLogoCG( CGContextRef c, const HIRect& r )
{
	// erase to background
	__Verify_noErr( HIThemeSetFill( kThemeBrushDialogBackgroundActive,
						nullptr, c, kHIThemeOrientationNormal ) );
	CGContextFillRect( c, r );
	
	// move to center of user-item, and scale so that LOGOBOX spans the entire height/width
	CGContextTranslateCTM( c, CGRectGetMidX( r ), CGRectGetMidY( r ) );
	CGContextScaleCTM( c, r.size.width / LOGOBOX, r.size.height / LOGOBOX );
	
	if ( true )	// B&W version
		{
/*
In the QuickDraw era, this was trivially easy: draw the main semicircle, then
draw another smaller circle on one side, and erase a similar circle on the other side.
Finally, XOR out the non-rotating triangle "delta" section.
But Quartz doesn't do XORs.
So what we do is this:
 1. clip to the _exterior_ of the triangle
 2. draw the positive part of the yinyang
 3. clip to the _interior_ of the triangle
 4. draw the positive yinyang, rotated 180 degrees for the inverted effect.
*/
		
		// color of the "positive" sections
		CGContextSetGrayFillColor( c, 0, 1 );	// black
		CGContextSaveGState( c );
		
			// clip to exterior of triangle: add outer rect (clockwise path)...
			CGContextBeginPath( c );
			CGContextAddRect( c, CGRectMake( -HALFBOX, -HALFBOX, LOGOBOX, LOGOBOX ) );
			// ... and the inner triangle (CCW path, ergo subtracted from clip).
			// Note that the y-axis is already flipped, providing a QD-style coordinate system.
			// Hence a negative Y-coord is _upward_, above the centerline.
			SetLogoTrianglePath( c, false );
			CGContextClip( c );
			
			// draw the parts of the logo that lie outside the triangle
			DrawYinYang( c, false );
		
		// pop & push the state, to reset the clipping path
		CGContextRestoreGState( c );
		CGContextSaveGState( c );
		
			// clip to _interior_ of triangle
			CGContextBeginPath( c );
			SetLogoTrianglePath( c, true );
			CGContextClip( c );
			
			// draw the bits inside the triangle (rotated 180 degrees)
			DrawYinYang( c, true );
		
		// restore state again
		CGContextRestoreGState( c );
		}
	else		// Technicolor version
		{
		// new, colorized scheme:
		// paint yin in 50% green and yang in 50% blue, both clipped to inside the triangle
		// then swap colors and paint them again, this time clipped to outside the triangle.
		
		// counter-rotate the triangle, 1/4 as fast as the YY's
		CGFloat angle = gLogoIndex * 2 * F_PI / (kNumLogoSteps * kCounterRotateRate);
		
		// also cycle the colors alluringly
		CGFloat primGreen = kYY_Secondary + (kYY_Range * ( (1.0F + sinf( angle )) / 2 ));
		CGFloat primBlue  = kYY_Secondary + (kYY_Range * ( (1.0F + cosf( angle )) / 2 ));
		
		CGContextSaveGState( c );
		
			// first clip to exterior of triangle
			CGContextRotateCTM( c, -angle );
			CGContextBeginPath( c );
			CGContextAddRect( c, CGRectMake( -HALFBOX, -HALFBOX, LOGOBOX, LOGOBOX ) );
			SetLogoTrianglePath( c, false );
			CGContextClip( c );
			CGContextRotateCTM( c, angle );
			
			// blue yin
			CGContextSetRGBFillColor( c, 0, kYY_Secondary, primBlue, kYY_Alpha );
			DrawYinYang( c, false );
			
			// green yang
			CGContextSetRGBFillColor( c, 0, primGreen, kYY_Secondary, kYY_Alpha );
			DrawYinYang( c, true );
		
		// reset the clipping path
		CGContextRestoreGState( c );
		CGContextSaveGState( c );
		
			// now clip to inside of triangle
			CGContextRotateCTM( c, -angle );	// note negated angle
			CGContextBeginPath( c );
			SetLogoTrianglePath( c, true );
			CGContextClip( c );
			CGContextRotateCTM( c, angle );	// undo rotation
			
			// green yin
			CGContextSetRGBFillColor( c, 0, primGreen, kYY_Secondary, kYY_Alpha );
			DrawYinYang( c, false );
			
			// blue yang
			CGContextSetRGBFillColor( c, 0, kYY_Secondary, primBlue, kYY_Alpha );
			DrawYinYang( c, true );
		
		CGContextRestoreGState( c );
		}
	
	// either way, lastly, stroke a circular thin black border around everything
	CGContextSetGrayStrokeColor( c, 0, 1 );
	CGContextSetLineWidth( c, kOuterStroke );
	CGContextMoveToPoint( c, RADIUS, 0 );	// positive X axis
	CGContextAddArc( c, 0, 0, RADIUS, 0, 2 * F_PI, 1 );	// a complete clockwise circle
	CGContextStrokePath( c );
}


/*
**	AB_HandleEvent()
**
**	handle click & keyboard events (to dismiss the dialog),
**	and control-draw events (to update the logo graphic)
*/
OSStatus
AB_HandleEvent( EventHandlerCallRef /*call*/, EventRef evt, void * ud )
{
	OSStatus result = eventNotHandledErr;
	UInt32 kind = GetEventKind( evt );
	UInt32 clas = GetEventClass( evt );
	
	if ( ( kEventClassMouse == clas && kEventMouseDown == kind )
	||   ( kEventClassKeyboard == clas && kEventRawKeyDown == kind ) )
		{
		__Verify_noErr( QuitAppModalLoopForWindow( GetSplashWindow() ) );
		result = noErr;
		}
	else
	if ( kEventClassControl == clas && kEventControlDraw == kind )
		{
		CGContextRef ctxt = nullptr;
		OSStatus err = GetEventParameter( evt, kEventParamCGContextRef, typeCGContextRef,
							nullptr, sizeof ctxt, nullptr, &ctxt );
		HIViewRef vu = nullptr;
		if ( noErr == err )
			(void) GetEventParameter( evt, kEventParamDirectObject, typeControlRef,
							nullptr, sizeof vu, nullptr, &vu );
		if ( ctxt && vu && vu == static_cast<HIViewRef>( ud ) )
			{
			HIRect box;
			__Verify_noErr( HIViewGetBounds( vu, &box ) );
			DrawLogoCG( ctxt, box );
			result = noErr;
			}
		}
	
	return result;
}

