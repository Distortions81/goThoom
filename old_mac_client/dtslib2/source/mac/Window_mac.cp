/*
**	Window_mac.cp		dtslib2
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

#include "Shell_mac.h"
#include "View_mac.h"
#include "Window_mac.h"

// #pragma GCC diagnostic warning "-Wdeprecated-declarations"


/*
**	Entry Routines
**
**	DTSWindow::DTSWindow();
**	DTSWindow::~DTSWindow();
**	DTSWindow::Init();
**	DTSWindow::SetTitle();
**	DTSWindow::Invalidate();
**	DTSWindow::GetMouseLoc();
**	DTSWindow::GetPosition();
**	DTSWindow::Close();
**	DTSWindow::Show();
**	DTSWindow::Hide();
**	DTSWindow::Move();
**	DTSWindow::Size();
**	DTSWindow::GoToFront();
**	DTSWindow::GoToBack();
**	DTSWindow::GoBehind();
**	DTSWindow::SetZoomSize();
**	DTSWindow::Hilite();
**	DTSWindow::MouseDown();
**	DTSWindow::Drag();
**	DTSWindow::Grow();
**	DTSWindow::GoAway();
**	DTSWindow::Zoom();
**	DTSWindow::Update();
**	
**	FindDTSWindow();
*/


/*
**	Internal Definitions
*/
const PropertyCreator	kDTSLibWindowCreator	= '\306Win';
const PropertyTag		kDTSLibWindowType		= '\306Win';

/*
**	Internal Routines
*/
//static void ClipGrowIcon( DTSWindow& window );


/*
**	DTSWindowPriv
*/
DTSDefineImplementFirmNoCopy(DTSWindowPriv)


/*
**	DTSWindowPriv::DTSWindowPriv()
*/
DTSWindowPriv::DTSWindowPriv() :
	winWindow( nullptr ),
//	winZoomMargin( 0, 0, 0, 0 ),
//	winUnzoomRect( 0, 0, 0, 0 ),
	winZoomWidth( 0 ),
	winZoomHeight( 0 ),
//	winHasGrowIcon( true ),
	winSaveVisible( false ),
	winIsFloater( false ),
	winGetFrontClick( false )
{
}


/*
**	DTSWindowPriv::~DTSWindowPriv()
*/
DTSWindowPriv::~DTSWindowPriv()
{
	if ( WindowRef win = winWindow )
		CFRelease( win );	// aka ReleaseWindow() aka DisposeWindow()
}


/*
**	DTSWindow::~DTSWindow()
*/
DTSWindow::~DTSWindow()
{
	Hide();
}


/*
**	GetContentView()
**
**	return the window's content view
*/
static HIViewRef
GetContentView( WindowRef w )
{
	HIViewRef vu = nullptr;
	if ( w )
		{
		__Verify_noErr( HIViewFindByID( HIViewGetRoot( w ), kHIViewWindowContentID, &vu ) );
		}
	return vu;
}


/*
**	DTSWindow::Init()
**
**	create a new Mac Window for this DTSWindow object
**	the 'kind' argument encodes the desired options & customizations
*/
DTSError
DTSWindow::Init( uint kind )
{
	DTSViewPriv * vp = DTSView::priv.p;
	if ( not vp )
		return -1;
	
	DTSWindowPriv * wp = DTSWindow::priv.p;
	if ( not wp )
		return -1;
	
	OSStatus err;
//	wp->winHasGrowIcon = false;
//	wp->winIsFloater   = false;
	
	// convert the kind to a window-class and attribute set
	WindowClass wClass = kDocumentWindowClass;
	WindowAttributes wAttrs = kWindowStandardDocumentAttributes;
	SInt16 winKind = kDocWinKind;
	
	switch ( kind & 0x07 )
		{
		case kWKindDocument:
//			wClass = kDocumentWindowClass;	// default
			break;
		
		case kWKindDlgBox:
			wClass = kMovableModalWindowClass;	// we're not fond of immovable-modal dialogs
			break;
		
		case kWKindPlainDlgBox:
			wClass = kPlainWindowClass;
			break;
		
		case kWKindMovableDlgBox:
			wClass = kMovableModalWindowClass;
			break;
		
		default:
			VDebugStr( "Invalid kind passed to DTSWindow." );
//			wClass = kDocumentWindowClass;
			break;
		}
	
	// translate DTSLib window-kinds to Mac window attributes
	if ( kind & kWKindFloater )
		{
		wClass = kFloatingWindowClass;
		wAttrs = kWindowStandardFloatingAttributes;
		wp->winIsFloater = true;
		winKind = kFloaterWinKind;
		}
	
	if ( kind & kWKindCloseBox )
		wAttrs |= kWindowCloseBoxAttribute;
	else
		wAttrs &= ~ kWindowCloseBoxAttribute;
	
	if ( kind & kWKindGrowBox )
		{
//		wp->winHasGrowIcon = true;
		wAttrs |= kWindowResizableAttribute;
		}
	else
		wAttrs &= ~ kWindowResizableAttribute;
	
	if ( kind & kWKindZoomBox )
		wAttrs |= kWindowFullZoomAttribute;
	else
		wAttrs &= ~ kWindowFullZoomAttribute;
	
	if ( kind & kWKindGetFrontClick )
		wp->winGetFrontClick = true;
	
	// floaters and dialogs don't appear in the Window menu
	if ( (kind & kWKindFloater) || (kind & 0x07) == kWKindMovableDlgBox )
		wAttrs &= ~ kWindowInWindowMenuAttribute;
	else
		wAttrs |= kWindowInWindowMenuAttribute;
	
	if ( kind & kWKindLiveResize )
		wAttrs |= kWindowLiveResizeAttribute;
	
	if ( kind & kWKindStandardHandler )
		wAttrs |= kWindowStandardHandlerAttribute;
	
	if ( kind & kWKindCompositing )
		{
		wAttrs |= kWindowCompositingAttribute;
		
		// hires mode unavailable on non-composited windows
		if ( kind & kWKindHighResolution )
			wAttrs |= kWindowFrameworkScaledAttribute;
		}
	
	// make sure everything's legit
	WindowAttributes avail = GetAvailableWindowAttributes( wClass );
	wAttrs &= avail;
	
	// make it
	WindowRef win;
	Rect bounds = { 100, 100, 200, 300 };
	err = CreateNewWindow( wClass, wAttrs, &bounds, &win );
	__Check_noErr( err );
	if ( noErr == err )
		{
		// mark it as ours
		DTSWindow * self = this;
		::SetWRefCon( win, reinterpret_cast<long>( this ) );
		__Verify_noErr( SetWindowProperty( win, kDTSLibWindowCreator, kDTSLibWindowType,
											sizeof self, &self ) );
		}
	else
		{
		SetErrorCode( err );
		return err;
		}
	
	if ( not win )
		return -1;
	
	wp->winWindow = win;
	::SetWindowKind( win, winKind );
	
	vp->InitViewPort( GetWindowPort( win ), nullptr );

#if 1
	// deactivate the root control if we're not frontmost; otherwise,
	// newly-made windows look active when they shouldn't.
	// The root control will be reactivated as soon as the window gets its first activate event.
	HIViewRef root = GetContentView( win );
	if ( root
	&&	 ActiveNonFloatingWindow() != win )
		{
		__Verify_noErr( HIViewSetActivated( root, false ) );
		}
#endif  // 0
	
	return noErr;
}


/*
**	FindDTSWindow()
**
**	Given an arbitrary WindowRef, return the corresponding DTSWindow object
**	(if there is one; otherwise NULL)
*/
DTSWindow *
FindDTSWindow( WindowRef macWin )
{
	DTSWindow * dtsWin = nullptr;
	if ( macWin )
		{
		OSStatus err = GetWindowProperty( macWin, kDTSLibWindowCreator, kDTSLibWindowType,
										sizeof dtsWin, nullptr, &dtsWin );
		if ( noErr != err )
			dtsWin = nullptr;
		}
	
	return dtsWin;
}


/*
**	DTSWindow::SetTitle()
**
**	set or change the window's title.
**  'title' is a MacRoman-encoded C string.
*/
void
DTSWindow::SetTitle( const char * title )
{
	DTSWindowPriv * p = priv.p;
	if ( not p )
		return;
	
	WindowRef win = p->winWindow;
	if ( not win )
		return;
	
	if ( CFStringRef cs = CreateCFString( title ) )
		{
		__Verify_noErr( SetWindowTitleWithCFString( win, cs ) );
		CFRelease( cs );
		}
}


/*
**	IsCompositingWindow()
**
**	returns true if a window is in compositing mode
*/
static inline bool
IsCompositingWindow( WindowRef w )
{
	// paranoia
	if ( not w )
		return false;
	
#if MAC_OS_X_VERSION_MIN_REQUIRED >= 1050
	return HIWindowTestAttribute( w, kHIWindowBitCompositing );
#else
	WindowAttributes attr = 0;
	if ( noErr == GetWindowAttributes( w, &attr ) )
		return attr & kWindowCompositingAttribute;
	
	return false;
#endif  // 10.5+
}


/*
**	DTSWindow::Invalidate()
**
**	mark the entire content region as needing to be redrawn
*/
void
DTSWindow::Invalidate()
{
	if ( DTSWindowPriv * p = priv.p )
		if ( WindowRef win = p->winWindow )
			{
			if ( not IsCompositingWindow( win ) )
				{
				Rect r;
				GetWindowPortBounds( win, &r );
				InvalWindowRect( win, &r );
				}
			}
}


/*
**	DTSWindow::Invalidate()
**
**	mark the given rectangle as needing to be redrawn
*/
void
DTSWindow::Invalidate( const DTSRect * box )
{
	if ( DTSWindowPriv * p = priv.p )
		if ( WindowRef win = p->winWindow )
			if ( not IsCompositingWindow( win ) )
				::InvalWindowRect( win, DTS2Mac(box) );
}


#ifndef DTS_XCODE
/*
**	DTSWindow::Invalidate()
**
**	mark the given region as needing to be redrawn
*/
void
DTSWindow::Invalidate( const DTSRegion * rgn )
{
	if ( DTSWindowPriv * p = priv.p )
		if ( WindowRef win = p->winWindow )
			if ( not IsCompositingWindow( win ) )
				::InvalWindowRgn( win, DTS2Mac(rgn) );
}
#endif  // DTS_XCODE


/*
**	DTSWindow::GetMouseLoc()
**
**	return mouse location in window-local coordinates
**	(content region starts at 0,0)
*/
void
DTSWindow::GetMouseLoc( DTSPoint * mouseLoc ) const
{
	if ( DTSWindowPriv * p = priv.p )
		if ( WindowRef win = p->winWindow )
			{
			if ( HIViewRef contVu = GetContentView( win ) )
				{
				HIPoint where;
#if MAC_OS_X_VERSION_MIN_REQUIRED >= 1050
				HIGetMousePosition( kHICoordSpaceView, contVu, &where );
#else
				Point globMouse;
				GetGlobalMouse( &globMouse );
				where = CGPointMake( globMouse.h, globMouse.v );
				HIPointConvert( &where, kHICoordSpace72DPIGlobal, nullptr,
										kHICoordSpaceView, contVu );
#endif  // 10.5+
				mouseLoc->Set( where.x, where.y );
				}
			}
}


/*
**	DTSWindow::GetPosition()
**
**	returns top left corner of window content in global coordinates
*/
void
DTSWindow::GetPosition( DTSPoint * pos ) const
{
	if ( DTSWindowPriv * p = priv.p )
		if ( WindowRef win = p->winWindow )
			{
#if 1
			Rect bounds;
			OSStatus err = GetWindowBounds( win, kWindowGlobalPortRgn, &bounds );
			__Check_noErr( err );
			if ( noErr == err )
				{
				pos->ptH = bounds.left;
				pos->ptV = bounds.top;
				}
			else
				pos->Set( 0, 0 );
#else
			StDrawEnv saveEnv( win );
			
			pos->Set( 0, 0 ); 
			::LocalToGlobal( DTS2Mac(pos) );
#endif  // 1
			}
}


/*
**	DTSWindow::Close()
**
**	close a window.  By default we just hide it; to close it permanently you'd need
**	to delete the DTSWindow object.  Subclasses can react differently, if they wish.
*/
void
DTSWindow::Close()
{
	Hide();
}


/*
**	DTSWindow::Show()
**
**	unhide the window (if it wasn't already visible)
*/
void
DTSWindow::Show()
{
	if ( DTSViewPriv * vp = DTSView::priv.p )
		if ( DTSWindowPriv * wp = DTSWindow::priv.p )
			if ( WindowRef win = wp->winWindow )
				if ( not vp->viewVisible )
					{
					vp->viewVisible = true;
					::ShowWindow( win );
					}
}


/*
**	DTSWindow::Hide()
**
**	hide the window (but don't remove it from memory)
*/
void
DTSWindow::Hide()
{
	if ( DTSViewPriv * vp = DTSView::priv.p )
		if ( DTSWindowPriv * wp = DTSWindow::priv.p )
			if ( WindowRef win = wp->winWindow )
				if ( vp->viewVisible )
					{
					vp->viewVisible    = false;
					wp->winSaveVisible = false;
					
					// we definitely cannot be the frontmost anymore
					if ( DTSWindow * front = gDTSApp->GetFrontWindow() )
						{
						DTSWindowPriv * fwp = front->priv.p;
						if ( fwp  &&  fwp->winWindow == win )
							{
							if ( DTSAppPriv * ap = gDTSApp->priv.p )
								ap->appFrontWin = nullptr;
							}
						}
					
					::HideWindow( win );
					}
}


/*
**	DTSWindow::Move()
**
**	reposition a DTSWindow, such that the top-left corner of its content region
**	is at (left, top) in global coordinates
*/
void
DTSWindow::Move( DTSCoord left, DTSCoord top )
{
	if ( DTSWindowPriv * p = priv.p )
		if ( WindowRef win = p->winWindow )
			::MoveWindow( win, left, top, false );
}


/*
**	DTSWindow::Size()
**
**	change the window's size
*/
void
DTSWindow::Size( DTSCoord width, DTSCoord height )
{
	if ( DTSView::priv.p )
		if ( DTSWindowPriv * wp = DTSWindow::priv.p )
			if ( WindowRef win = wp->winWindow )
				{
//				StDrawEnv save( this );
				
				Rect bounds;
				GetWindowBounds( win, kWindowContentRgn, &bounds );
//				::GetWindowPortBounds( win, &bounds );
				
				if ( height != bounds.bottom - bounds.top
				||   width  != bounds.right - bounds.left )
					{
					// Mac calls to size the window
#if 1
					// invalidates just the newly-exposed parts
					::SizeWindow( win, width, height, true );
#else
					::SizeWindow( win, width, height, false );
					
					// invalidate the whole window
					::GetWindowPortBounds( win, &bounds );
					::InvalWindowRect( win, &bounds );
#endif
					}
				
				// call through to the parent class
				// this sets viewBounds and viewContent
				DTSView::Size( width, height );
				
				// draw the grow icon
//				ClipGrowIcon( *this );
				}
}


/*
**	DTSWindow::GoToFront()
**
**	send one DTSWindow to the front
*/
void
DTSWindow::GoToFront()
{
	if ( DTSWindowPriv * p = priv.p )
		if ( WindowRef win = p->winWindow )
			{
			WindowRef select = ::ActiveNonFloatingWindow();
			if ( select != win )
				::SelectWindow( win );
			}
}


/*
**	DTSWindow::GoToBack()
**
**	send one DTSWindow behind all others
*/
void
DTSWindow::GoToBack()
{
	if ( DTSWindowPriv * p = priv.p )
		if ( WindowRef win = p->winWindow )
			{
			::SendBehind( win, kLastWindowOfClass );
			ActivateWindows();
			}
}


/*
**	DTSWindow::GoBehind()
**
**	send one DTSWindow behind another
*/
void
DTSWindow::GoBehind( const DTSWindow * behind )
{
	if ( DTSWindowPriv * p = priv.p )
		if ( WindowRef win = p->winWindow )
			{
			::SendBehind( win, DTS2Mac(behind) );
			ActivateWindows();
			}
}


/*
**	DTSWindow::SetZoomSize()
**
**	update the window's notion of how big its "ideal standard state" should be,
**	for the next time it gets zoomed out.
*/
void
DTSWindow::SetZoomSize( DTSCoord width, DTSCoord height )
{
	if ( DTSWindowPriv * p = priv.p )
		{
		p->winZoomWidth  = width;
		p->winZoomHeight = height;
		}
}


#if 0
/*
**	DTSWindow::ClipGrowIcon()
*/
void
ClipGrowIcon( DTSWindow& window )
{
	if ( DTSViewPriv * vp = window.DTSView::priv.p )
		if ( DTSWindowPriv * wp = window.DTSWindow::priv.p )
			if ( wp->winWindow && wp->winHasGrowIcon )
				{
				StDrawEnv save( &window );
				
				// update the window's clipping region
				::SetClip( DTS2Mac(&vp->viewContent) );
				}
}


/*
**	DTSWindow_Hilite()
**
**	we got an activate or deactivate event for this window
*/
void
DTSWindow_Hilite( DTSWindow& /* window */ )
{
	ClipGrowIcon( window );
}
#endif  // 0


/*
**	DTSWindow_MouseDown()
**
**	we got a mouseDown event inside a window's inContent region
*/
void
DTSWindow_MouseDown( DTSWindow& window, const DTSPoint& where, ulong when, uint modifiers )
{
	DTSWindowPriv * p = window.priv.p;
	if ( not p )
		return;
	
	WindowRef win = p->winWindow;
	if ( not win )
		return;
	
	StDrawEnv save( &window );
	
	// convert 'where' from global to content-local coordinates
#if 1
	HIViewRef vu = GetContentView( win );
	HIPoint pt = CGPointMake( where.ptH, where.ptV );
	HIPointConvert( &pt, kHICoordSpace72DPIGlobal, nullptr, kHICoordSpaceView, vu );
	DTSPoint localMouse( pt.x, pt.y );
#else
	DTSPoint localMouse = where;
	QDGlobalToLocalPoint( GetWindowPort( win ), DTS2Mac( &localMouse ) );
#endif
	
	if ( kFloaterWinKind == ::GetWindowKind( win ) )
		{
		// a click in a floating window brings it to the front,
		// and is passed to the subclass always (regardless of winGetFrontClick)
		window.GoToFront();
		window.Click( localMouse, when, modifiers );
		}
	else
	if ( gDTSApp->GetFrontWindow() != &window )
		{
		// a click in non-front, non-floating window brings it to front...
		window.GoToFront();
		
		// ... but such clicks are not further processed unless specifically requested
		if ( p->winGetFrontClick )
			{
			// not sure we still need this
			if ( not IsCompositingWindow( win ) )
				::SetPortWindowPort( win );	// GoToFront() does NOT restore port!
			
			window.Click( localMouse, when, modifiers );
			}
		}
	else
		{
		// simple case: a click in a frontmost, non-floating window
		DTSWindow_Update( window );
		window.Click( localMouse, when, modifiers );
		}
}


/*
**	DTSWindow_Drag()
**
**	we got a mouseDown event inside a window's inDrag region
*/
void
DTSWindow_Drag( DTSWindow& window, const DTSPoint& where, bool dontComeFront )
{
	DTSWindowPriv * p = window.priv.p;
	if ( not p )
		return;
	
	if ( not /* (gEvent.modifiers & cmdKey)*/ dontComeFront )
		window.GoToFront();
	
//	StDrawEnv save( &window );
	
	WindowRef win = p->winWindow;
	::DragWindow( win, *DTS2Mac(&where), nullptr /* DTS2Mac(&gDragLimit) */ );
}


/*
**	DTSWindow_Grow()
**
**	we got a mouseDown event inside a window's inGrow region
*/
void
DTSWindow_Grow( DTSWindow& window, const DTSPoint& where )
{
	DTSWindowPriv * p = window.priv.p;
	if ( not p )
		return;
	
	Rect r;
	bool changed = ::ResizeWindow( p->winWindow, *DTS2Mac(&where), nullptr, &r );
	if ( changed )
		{
		StDrawEnv save( &window );
		window.Size( r.right - r.left, r.bottom - r.top );
		}
}


/*
**	DTSWindow_GoAway()
**
**	we got a mouseDown event inside a window's inGoAway region
*/
void
DTSWindow_GoAway( DTSWindow& window, const DTSPoint& where )
{
	DTSWindowPriv * p = window.priv.p;
	if ( not p )
		return;
	
	WindowRef win = p->winWindow;
	if ( not win )
		return;
	
	// track the go away box
	if ( ::TrackGoAway( win, *DTS2Mac(&where) ) )
		{
		StDrawEnv save( &window );
		
		if ( window.CanClose() )
			window.Close();
		}
}


/*
**	DTSWindow_Zoom()
**
**	we got a mouseDown event inside a window's inZoomIn or inZoomOut regions
*/
void
DTSWindow_Zoom( DTSWindow& window, const DTSPoint& where, int partCode )
{
	DTSWindowPriv * p = window.priv.p;
	if ( not p )
		return;
	
	WindowRef win = p->winWindow;
	if ( not win )
		return;
	
	// track the zoom box
	// bail if the user releases outside the box
	bool bResult = ::TrackBox( win, *DTS2Mac(&where), partCode );
	if ( not bResult )
		return;
	
	// ZoomWindowIdeal() handles all the messy crap that we used to
	// have to deal with manually, back in the ZoomWindow() days
	Point idealSize = { p->winZoomHeight, p->winZoomWidth };
	Rect idealBox;
	if ( idealSize.h <= 0 || idealSize.v <= 0 )
		bResult = IsWindowInStandardState( win, nullptr, &idealBox );
	else
		bResult = IsWindowInStandardState( win, &idealSize, &idealBox );
	
	if ( not bResult )	// -> inZoomOut
		{
		idealSize.v = idealBox.bottom - idealBox.top;
		idealSize.h = idealBox.right - idealBox.left;
		}
	OSStatus err = ZoomWindowIdeal( win, bResult ? inZoomIn : inZoomOut, &idealSize );
	__Check_noErr( err );
	if ( noErr == err )
		{
		StDrawEnv save( &window );
		
		Rect b;
		__Verify_noErr( GetWindowBounds( win, kWindowContentRgn, &b ) );
		window.Size( b.right - b.left, b.bottom - b.top );
		}
}


/*
**	DTSWindow_Update()
**
**	we got an update event for a DTSWindow
*/
void
DTSWindow_Update( DTSWindow& window )
{
	DTSWindowPriv * p = window.priv.p;
	if ( not p )
		return;
	
	WindowRef win = p->winWindow;
	if ( not win )
		return;
	
	// just in case
	if ( ::IsWindowUpdatePending( win ) ) 
		{
		StDrawEnv saveEnv( &window );
		
		::BeginUpdate( win );
//		ClipGrowIcon( window );
		
		window.DoDraw();
		
		::EndUpdate( win );
		}
}


/*
**	DTSWindow::IsMinimized()
*/
bool
DTSWindow::IsMinimized() const
{
	bool result = false;
	if ( DTSWindowPriv * p = priv.p )
		if ( WindowRef win = p->winWindow )
			result = ::IsWindowCollapsed( win );
	
	return result;
}


/*
**	DTSWindow::SetMinimized()
*/
DTSError
DTSWindow::SetMinimized( bool minimize )
{
	OSStatus result = -1;
	if ( DTSWindowPriv * p = priv.p )
		if ( WindowRef win = p->winWindow )
			{
			result = ::CollapseWindow( win, minimize );
			__Check_noErr( result );
			}
	
	return result;
}


/*
**	DTSWindow::CanClose()
*/
bool
DTSWindow::CanClose()
{
	return true;
}


/*
**	DTSWindow::Activate()
*/
void
DTSWindow::Activate()
{
}


/*
**	DTSWindow::Deactivate()
*/
void
DTSWindow::Deactivate()
{
}


/*
**	DTSWindow::DoCmd()
*/
bool
DTSWindow::DoCmd( long /* cmd */ )
{
	return false;	// not handled
}


