/*
**	View_mac.cp		dtslib2
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

// #pragma GCC diagnostic warning "-Wdeprecated-declarations"

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "Memory_dts.h"
#include "Shell_mac.h"
#include "View_mac.h"


/*
**	Entry Routines
**
**	DTSView
**	DTSOffView
*/


/*
**	Internal Routines
*/
#if 0
static Boolean			IndexSearchProc( RGBColor * rgb, long * position );
static ColorSearchUPP	GetIndexSearchUPP(); 	
#endif  // 0


/*
**	Internal Variables
*/
#ifndef AVOID_QUICKDRAW
static const char	shiftTable[] = {
							0,0,
							1,0,
							2,0,0,0,
							3,0,0,0,0,0,0,0,
							4,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,
							5 };

static PixMapPtr	cacheMap  = nullptr;
static uchar *		cacheAddr = nullptr;

static GDHandle		gGDevice1;
static GDHandle		gGDevice8;
static GDHandle		gGDevice16;
static GDHandle		gGDevice32;
#endif  // ! AVOID_QUICKDRAW


#if defined( DEBUG_VERSION ) && ! defined( AVOID_QUICKDRAW )
/*
**	CheckPort()
**
**	debug sanity checker
*/
static void
CheckPort( const DTSView& view, int lineNo )
{
	// get Quickdraw's current port
	GrafPtr theQDPort = ::GetQDGlobalsThePort();
	
	DTSViewPriv * p = view.priv.p;
	if ( not p )
		return;
	
	void * port = p->viewPort;
	if ( port != theQDPort )
		{
		VDebugStr( "View is not current grafport! (" __FILE__ ", line %d)", lineNo );
		
		// fix things enough so we can limp onward
		::SetPort( static_cast<GrafPtr>( port ) );
		}
}
#endif	// DEBUG_VERSION && ! AVOID_QUICKDRAW


/*
**	CheckDTSView()
**
**	zero overhead for non-debug builds
*/
#if defined( DEBUG_VERSION ) && ! defined( AVOID_QUICKDRAW )
# define CheckDTSView()	CheckPort( *this, __LINE__ )
#else
# define CheckDTSView()
#endif


/*
**	DTSViewPriv
*/
DTSDefineImplementFirmNoCopy(DTSViewPriv)


/*
**	DTSViewPriv::DTSViewPriv()
*/
DTSViewPriv::DTSViewPriv() :
		viewPort( nullptr ),
		viewDevice( nullptr ),
		viewParent( nullptr ),
		viewChild( nullptr ),
		viewSibling( nullptr ),
		viewBounds( 0, 0, 0, 0 ),
		viewVisible( false )
{
#ifndef AVOID_QUICKDRAW
	viewContent.SetEmpty();
#endif
}


/*
**	DTSViewPriv::~DTSViewPriv()
*/
DTSViewPriv::~DTSViewPriv()
{
	Detach();
}


/*
**	DTSView::Attach()
*/
void
DTSView::Attach( DTSView * parent )
{
	if ( not parent )
		return;
	DTSViewPriv * p = priv.p;
	if ( not p )
		return;
	DTSViewPriv * parentp = parent->priv.p;
	if ( not parentp )
		return;
	
	// make sure we don't already have a parent
	if ( p->viewParent )
		{
#ifdef DEBUG_VERSION
		VDebugStr( "View already has parent." );
#endif
		return;
		}
	
	// inherit the parent's port and device
	p->viewPort   = parentp->viewPort;
	p->viewDevice = parentp->viewDevice;
	
	// add ourselves to the parent's list
	p->viewParent  = parent;
	p->viewSibling = parentp->viewChild;
	parentp->viewChild = this;
}


/*
**	DTSView::Detach()
*/
void
DTSView::Detach()
{
	if ( DTSViewPriv * p = priv.p )
		p->Detach();
}


/*
**	DTSViewPriv::Detach()
*/
void
DTSViewPriv::Detach()
{
	// remove ourselves from our parent
	DTSView * view = viewParent;
	if ( not view )
		return;
	
	DTSViewPriv * p = view->priv.p;
	if ( not p )
		return;
	
	for ( DTSView ** link = &p->viewChild; ; link = &p->viewSibling )
		{
		view = *link;
		if ( not view )
			{
#ifdef DEBUG_VERSION
			VDebugStr( "View not found in parent's list" );
#endif
			break;
			}
		
		p = view->priv.p;
		if ( not p )
			break;
		
		if ( p == this )
			{
			*link = p->viewSibling;
			break;
			}
		}
	/*p->*/ viewParent = nullptr;
}


/*
**	DTSView::Reset()
*/
void
DTSView::Reset()
{
#ifndef AVOID_QUICKDRAW
	DTSViewPriv * p = priv.p;
	if ( not p )
		return;
	
	CheckDTSView();
	SetMask( &p->viewContent );
	
	::PenNormal();
	
	// forecolor black
	::ForeColor( blackColor );
	
	// backcolor white
	::BackColor( whiteColor );
	
	::TextFont( 0 );
	::TextSize( 0 );
	::TextFace( 0 );
	
	// opcolor intermediate
	const RGBColor color = { 0x8000, 0x8000, 0x8000 };
	::OpColor( &color );
#endif  // ! AVOID_QUICKDRAW
}


#ifndef AVOID_QUICKDRAW
/*
**	DTSView::GetMask()
*/
void
DTSView::GetMask( DTSRegion * rgn ) const
{
	CheckDTSView();
	::GetClip( DTS2Mac(rgn) );
}


/*
**	DTSView::SetMask()
*/
void
DTSView::SetMask( const DTSRegion * rgn )
{
	CheckDTSView();
	::SetClip( DTS2Mac(rgn) );
}
#endif  // ! AVOID_QUICKDRAW


#ifndef AVOID_QUICKDRAW
/*
**	DTSView::SetMask()
*/
void
DTSView::SetMask( const DTSRect * rect )
{
	CheckDTSView();
	::ClipRect( DTS2Mac(rect) );
}
#endif  // ! AVOID_QUICKDRAW


/*
**	DTSView::SetPenSize()
*/
void
DTSView::SetPenSize( DTSCoord sz )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	::PenSize( sz, sz );
#else
# pragma unused( sz )
#endif
}


/*
**	DTSView::SetPenMode()
*/
void
DTSView::SetPenMode( int mode )
{
	DTSViewPriv * p = priv.p;
	if ( not p )
		return;
	
	GrafPtr port = p->viewPort;
	if ( not port )
		return;
	
#ifndef AVOID_QUICKDRAW
	// map DTS modes to QD modes
	switch ( mode )
		{
		case kPenModeTransparent:
			mode = transparent;			// always a pixmap under Carbon
			break;
		
		case kPenModeInvert:
			mode = patXor;
			break;
		
		case kPenModeErase:
			mode = patBic;
			break;
		
		case kPenModeBlend:
			mode = blend;
			break;
		
		//case kPenModeCopy:
		default:
			mode = patCopy;
			break;
		}
	
	CheckDTSView();
	::PenMode( mode );
#else
# pragma unused( mode )
#endif  // ! AVOID_QUICKDRAW
}


#ifndef AVOID_QUICKDRAW
/*
**	DTSView::SetPattern()
*/
void
DTSView::SetPattern( const DTSPattern * pat )
{
	CheckDTSView();
	PixPatHandle hdl = nullptr;
	DTSPatternPriv * patPriv = pat->priv.p;
	if ( patPriv )
		hdl = patPriv->patHdl;
	if ( hdl )
		::PenPixPat( hdl );
	else
		::PenPat( &patPriv->pattern );
}
#endif  // ! AVOID_QUICKDRAW


/*
**	DTSView::SetForeColor()
*/
void
DTSView::SetForeColor( const DTSColor * color )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	::RGBForeColor( DTS2Mac(color) );
#else
# pragma unused( color )
#endif  // ! AVOID_QUICKDRAW
}


#if 0
/*
**	DTSView::SetForeColor()
**	by index
*/
void
DTSView::SetForeColor( int index )
{
	CheckDTSView();
	::AddSearch( GetIndexSearchUPP() );
	
	RGBColor color;
	* (int32_t *) &color = index;
	::RGBForeColor( &color );
	
	::DelSearch( GetIndexSearchUPP() );
}
#endif  // 0


/*
**	DTSView::SetBackColor()
*/
void
DTSView::SetBackColor( const DTSColor * color )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	::RGBBackColor( DTS2Mac(color) );
#else
# pragma unused( color )
#endif
}


#if 0
/*
**	DTSView::SetBackColor()
**	by index
*/
void
DTSView::SetBackColor( int index )
{
	CheckDTSView();
	::AddSearch( GetIndexSearchUPP() );
	
	RGBColor color;
	* (int32_t *) &color = index;
	::RGBBackColor( &color );
	
	::DelSearch( GetIndexSearchUPP() );
}
#endif  // 0


/*
**	DTSView::SetHiliteColor()
*/
void
DTSView::SetHiliteColor( const DTSColor * color )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	::HiliteColor( DTS2Mac(color) );
#else
# pragma unused( color )
#endif
}


#if 0
/*
**	DTSView::SetHiliteColor()
**	by index
*/
void
DTSView::SetHiliteColor( int index )
{
	CheckDTSView();
	::AddSearch( GetIndexSearchUPP() );
	
	RGBColor color;
	* (int32_t *) &color = index;
	::HiliteColor( &color );
	
	::DelSearch( GetIndexSearchUPP() );
}


/*
**	IndexSearchProc()
*/
Boolean
IndexSearchProc( RGBColor * color, long * position )
{
	*position = * (int32_t *) color;
	return true;
}


/*
**	GetIndexSearchUPP()
**
**	caches a UPP for the index searches
*/
ColorSearchUPP
GetIndexSearchUPP()
{
	static ColorSearchUPP upp;
	if ( not upp )
		{
		upp = ::NewColorSearchUPP( IndexSearchProc );
		__Check( upp );
		}
	return upp;
}
#endif  // 0


/*
**	DTSView::SetHiliteMode()
**
**	tell ColorQD to do the next inversion in the user's hilite color
*/
void
DTSView::SetHiliteMode()
{
#ifndef AVOID_QUICKDRAW
	UInt8 mode = ::LMGetHiliteMode();
	::LMSetHiliteMode( mode & 0x07F );
#endif
}


/*
**	DTSView::SetBlend()
*/
void
DTSView::SetBlend( int value )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	RGBColor color = { value, value, value };
	::OpColor( &color );
#else
# pragma unused( value )
#endif
}


/*
**	DTSView::SetFont()
*/
void
DTSView::SetFont( const char * fontName )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	
	Str255 pFontName;
	myctopstr( fontName, pFontName );
	
	SInt16 id = ::FMGetFontFamilyFromName( pFontName );
//	::GetFNum( pFontName, &id );
	::TextFont( id );
#else
# pragma unused( fontName )
#endif
}


/*
**	DTSView::SetFontSize()
*/
void
DTSView::SetFontSize( DTSCoord size )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	::TextSize( size );
#else
# pragma unused( size )
#endif
}


/*
**	DTSView::SetFontStyle()
*/
void
DTSView::SetFontStyle( int style )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	::TextFace( style );
#else
# pragma unused( style )
#endif
}


/*
**	DTSView::GetTextWidth()
*/
DTSCoord
DTSView::GetTextWidth( const char * text ) const
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	int len = std::strlen( text );
	if ( len >= SHRT_MAX )
		len = SHRT_MAX - 1;
	return ::TextWidth( text, 0, len );
#else
# pragma unused( text )
	return 0;
#endif
}


/*
**	DTSView::Frame()
*/
void
DTSView::Frame( const DTSRect * rect )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	::FrameRect( DTS2Mac(rect) );
#else
# pragma unused( rect )
#endif
}


/*
**	DTSView::Frame()
*/
void
DTSView::Frame( const DTSOval * oval )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	::FrameOval( DTS2Mac(oval) );
#else
# pragma unused( oval )
#endif
}


/*
**	DTSView::Frame()
*/
void
DTSView::Frame( const DTSRound * round )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	::FrameRoundRect( DTS2Mac(round), round->roundWidth, round->roundHeight );
#else
# pragma unused( round )
#endif
}


/*
**	DTSView::Frame()
*/
void
DTSView::Frame( const DTSArc * arc )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	int start = arc->arcStart;
	::FrameArc( DTS2Mac(arc), start, ( arc->arcStop - start ) % 360 );
#else
# pragma unused( arc )
#endif
}


#if 0
/*
**	DTSView::Frame()
*/
void
DTSView::Frame( const DTSPolygon * poly )
{
	CheckDTSView();
	::FramePoly( DTS2Mac(poly) );
}
#endif  // 0


#ifndef AVOID_QUICKDRAW
/*
**	DTSView::Frame()
*/
void
DTSView::Frame( const DTSRegion * rgn )
{
	CheckDTSView();
	::FrameRgn( DTS2Mac(rgn) );
}
#endif  // ! AVOID_QUICKDRAW


/*
**	DTSView::Paint()
*/
void
DTSView::Paint( const DTSRect * rect )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	::PaintRect( DTS2Mac(rect) );
#else
# pragma unused( rect )
#endif
}


/*
**	DTSView::Paint()
*/
void
DTSView::Paint( const DTSOval * oval )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	::PaintOval( DTS2Mac(oval) );
#else
# pragma unused( oval )
#endif
}


/*
**	DTSView::Paint()
*/
void
DTSView::Paint( const DTSRound * round )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	::PaintRoundRect( DTS2Mac(round), round->roundWidth, round->roundHeight );
#else
# pragma unused( round )
#endif
}


/*
**	DTSView::Paint()
*/
void
DTSView::Paint( const DTSArc * arc )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	int start = arc->arcStart;
	::PaintArc( DTS2Mac(arc), start, ( arc->arcStop - start ) % 360 );
#else
# pragma unused( arc )
#endif
}


#if 0
/*
**	DTSView::Paint()
*/
void
DTSView::Paint( const DTSPolygon * poly )
{
	CheckDTSView();
	::PaintPoly( DTS2Mac(poly) );
}
#endif  // 0


#ifndef AVOID_QUICKDRAW
/*
**	DTSView::Paint()
*/
void
DTSView::Paint( const DTSRegion * rgn )
{
	CheckDTSView();
	::PaintRgn( DTS2Mac(rgn) );
}
#endif  // ! AVOID_QUICKDRAW


/*
**	DTSView::Erase()
*/
void
DTSView::Erase( const DTSRect * rect )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	::EraseRect( DTS2Mac(rect) );
#else
# pragma unused( rect )
#endif
}


/*
**	DTSView::Erase()
*/
void
DTSView::Erase( const DTSOval * oval )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	::EraseOval( DTS2Mac(oval) );
#else
# pragma unused( oval )
#endif
}


/*
**	DTSView::Erase()
*/
void
DTSView::Erase( const DTSRound * round )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	::EraseRoundRect( DTS2Mac(round), round->roundWidth, round->roundHeight );
#else
# pragma unused( round )
#endif
}


/*
**	DTSView::Erase()
*/
void
DTSView::Erase( const DTSArc * arc )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	int start = arc->arcStart;
	::EraseArc( DTS2Mac(arc), start, ( arc->arcStop - start ) % 360 );
#else
# pragma unused( arc )
#endif
}


#if 0
/*
**	DTSView::Erase()
*/
void
DTSView::Erase( const DTSPolygon * poly )
{
	CheckDTSView();
	::ErasePoly( DTS2Mac(poly) );
}
#endif  // 0


#ifndef AVOID_QUICKDRAW
/*
**	DTSView::Erase()
*/
void
DTSView::Erase( const DTSRegion * rgn )
{
	CheckDTSView();
	::EraseRgn( DTS2Mac(rgn) );
}
#endif  // ! AVOID_QUICKDRAW


/*
**	DTSView::Invert()
*/
void
DTSView::Invert( const DTSRect * rect )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	::InvertRect( DTS2Mac(rect) );
#else
# pragma unused( rect )
#endif
}


/*
**	DTSView::Invert()
*/
void
DTSView::Invert( const DTSOval * oval )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	::InvertOval( DTS2Mac(oval) );
#else
# pragma unused( oval )
#endif
}


/*
**	DTSView::Invert()
*/
void
DTSView::Invert( const DTSRound * round )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	::InvertRoundRect( DTS2Mac(round), round->roundWidth, round->roundHeight );
#else
# pragma unused( round )
#endif
}


/*
**	DTSView::Invert()
*/
void
DTSView::Invert( const DTSArc * arc )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	int start = arc->arcStart;
	::InvertArc( DTS2Mac(arc), start, ( arc->arcStop - start ) % 360 );
#else
# pragma unused( arc )
#endif
}


#if 0
/*
**	DTSView::Invert()
*/
void
DTSView::Invert( const DTSPolygon * poly )
{
	CheckDTSView();
	::InvertPoly( DTS2Mac(poly) );
}
#endif  // 0


#ifndef AVOID_QUICKDRAW
/*
**	DTSView::Invert()
*/
void
DTSView::Invert( const DTSRegion * rgn )
{
	CheckDTSView();
	::InvertRgn( DTS2Mac(rgn) );
}
#endif  // ! AVOID_QUICKDRAW


/*
**	DTSView::Draw()
**	draw a straight line
*/
void
DTSView::Draw( const DTSLine * line )
{
#ifndef AVOID_QUICKDRAW
	CheckDTSView();
	::MoveTo( line->linePt1.ptH, line->linePt1.ptV );
	::LineTo( line->linePt2.ptH, line->linePt2.ptV );
#else
# pragma unused( line )
#endif
}


#ifndef AVOID_QUICKDRAW
/*
**	DTSView::Draw()
**	blit an image
*/
void
DTSView::Draw( const DTSImage * image, int mode, const DTSRect * src, const DTSRect * dst )
{
	DTSViewPriv * p = priv.p;
	if ( not p )
		return;
	
	GrafPtr port = p->viewPort;
	if ( not port )
		return;
	
	CheckDTSView();
	// check for an uninitialized image
	if ( not image
	||   not image->GetBits() )
		{
		VDebugStr( "Warning: Bad DTSImage passed to DTSView::Draw." );
		return;
		}
	
	// translate the DTS mode to a QuickDraw mode
	int qdMode = srcCopy;
	switch ( mode )
		{
		case kImageCopyQuality:
			qdMode = ditherCopy;
			break;
		
		case kImageTransparent:
			qdMode = transparent;
			break;
		
		case kImageErase:
			qdMode = srcBic;
			break;
		
		case kImageBlend:
			qdMode = blend;
			break;
		
		case kImageTransparentQuality:
			qdMode = transparent | ditherCopy;
		}
	
	const BitMap * dstP = ::GetPortBitMapForCopyBits( port );
	// copy the bits
	::CopyBits(
		DTS2Mac(image),
		dstP,
		DTS2Mac(src),
		DTS2Mac(dst),
		qdMode, nullptr );
}
#endif  // ! AVOID_QUICKDRAW


/*
**	DTSView::Draw()
**
**	blit one DTSView into another
*/
void
DTSView::Draw( const DTSView * view, int mode, const DTSRect * src, const DTSRect * dst )
{
#ifdef AVOID_QUICKDRAW
# pragma unused( view, mode, src, dst )
#else
	if ( not view )
		return;
	
	DTSViewPriv * p = view->priv.p;
	if ( not p )
		return;
	
	// sanity check
	CheckDTSView();
	GrafPtr port = p->viewPort;
	if ( not port )
		return;
	
	// build a DTSImage from the portbits of view's pixmap
	DTSImage image;
	DTSImagePriv * ip = image.priv.p;
	if ( not ip )
		return;
	
	// get a pointer to the pixmap
	ip->imageMap = ** ::GetPortPixMap( port );
	
	// draw the darn thing
	Draw( &image, mode, src, dst );
#endif  // 10.8
}


/*
**	DTSView::Draw()
**
**	draw some text
*/
void
DTSView::Draw( const char * text, DTSCoord h, DTSCoord v, int just )
{
#ifdef AVOID_QUICKDRAW
# pragma unused( text, h, v, just )
#else
	CheckDTSView();
	int length = std::strlen( text );
	if ( length >= SHRT_MAX )
		length = SHRT_MAX - 1;
	
	if ( just != kJustLeft )
		{
		int width = ::TextWidth( text, 0, length );
		
		if ( just != kJustRight )
			width /= 2;
		h -= width;
		}
	
	::MoveTo( h, v );
	::DrawText( text, 0, length );
#endif  // AVOID_QUICKDRAW
}


#ifndef AVOID_QUICKDRAW
/*
**	DTSView::Draw()
**
**	draw a picture
*/
void
DTSView::Draw( const DTSPicture * picture, const DTSRect * bounds )
{
	CheckDTSView();
	::DrawPicture( DTS2Mac(picture), DTS2Mac(bounds) );
}
#endif  // ! AVOID_QUICKDRAW


/*
**	DTSView::Show()
*/
void
DTSView::Show()
{
	DTSViewPriv * p = priv.p;
	if ( not p )
		return;
	
	if ( not p->viewVisible )
		{
		p->viewVisible = true;
		Draw();
		}
}


/*
**	DTSView::Hide()
*/
void
DTSView::Hide()
{
	if ( DTSViewPriv * p = priv.p )
		p->viewVisible = false;
}


/*
**	DTSView::Draw()
*/
void
DTSView::Draw()
{
	DTSViewPriv * p = priv.p;
	if ( not p )
		return;
	
	GrafPtr port = p->viewPort;
	if ( port
	&&   p->viewVisible )
		{
		StDrawEnv saveEnv( port, p->viewDevice );
		
		DoDraw();
		}
}


/*
**	DTSView::DoDraw()
*/
void
DTSView::DoDraw()
{
	DTSViewPriv * p = priv.p;
	if ( not p )
		return;
	
	DTSViewPriv * childp;
	for ( DTSView * child = p->viewChild;  child;  child = childp->viewSibling )
		{
		childp = child->priv.p;
		if ( not childp )
			break;
		
		if ( childp->viewVisible )
			child->DoDraw();
		}
}


/*
**	DTSView::Move()
*/
void
DTSView::Move( DTSCoord h, DTSCoord v )
{
	if ( DTSViewPriv * p = priv.p )
		{
		p->viewBounds.Move( h, v );
#ifndef AVOID_QUICKDRAW
		p->viewContent.Set( &p->viewBounds );
#endif
		}
}


/*
**	DTSView::Size()
*/
void
DTSView::Size( DTSCoord width, DTSCoord height )
{
	if ( DTSViewPriv * p = priv.p )
		{
		p->viewBounds.Size( width, height );
#ifndef AVOID_QUICKDRAW
		p->viewContent.Set( &p->viewBounds );
#endif
		}
}


/*
**	DTSView::Click()
*/
bool
DTSView::Click( const DTSPoint& loc, ulong when, uint modifiers )
{
	DTSViewPriv * p = priv.p;
	if ( not p )
		return false;
	
	DTSViewPriv * viewp;
	for ( DTSView * view = p->viewChild;  view;  view = viewp->viewSibling )
		{
		viewp = view->priv.p;
		if ( not viewp )
			break;
		
		if ( viewp->viewVisible
#ifndef AVOID_QUICKDRAW
		&&   loc.InRegion( &viewp->viewContent )
#endif
		)
			{
			if ( view->Click( loc, when, modifiers ) )
				return true;
			}
		}
	
	return false;
}


/*
**	DTSViewPriv::InitViewPort()
*/
void
DTSViewPriv::InitViewPort( GrafPtr port, GDHandle device )
{
#ifndef AVOID_QUICKDRAW
	if ( not device )
		device = ::GetMainDevice();
	
	viewDevice = device;
#else
# pragma unused( device )
#endif
	
	viewPort   = port;
}


/*
**	DTSView::GetParent()
*/
DTSView *
DTSView::GetParent() const
{
	const DTSViewPriv * p = priv.p;
	return p ? p->viewParent : nullptr;
}


/*
**	DTSView::GetChild()
*/
DTSView *
DTSView::GetChild() const
{
	const DTSViewPriv * p = priv.p;
	return p ? p->viewChild : nullptr;
}


/*
**	DTSView::GetSibling()
*/
DTSView *
DTSView::GetSibling() const
{
	const DTSViewPriv * p = priv.p;
	return p ? p->viewSibling : nullptr;
}


/*
**	DTSView::GetBounds()
*/
void
DTSView::GetBounds( DTSRect * bounds ) const
{
	if ( DTSViewPriv * p = priv.p )
		*bounds = p->viewBounds;
	else
		bounds->Set( 0x8000, 0x8000, 0x8000, 0x8000 );
}


/*
**	DTSView::GetVisible()
*/
bool
DTSView::GetVisible() const
{
	const DTSViewPriv * p = priv.p;
	return p ?  p->viewVisible : false;
}


#ifndef AVOID_QUICKDRAW
/*
**	DTSView::GetContent()
*/
void
DTSView::GetContent( DTSRegion * rgn ) const
{
	if ( DTSViewPriv * p = priv.p )
		rgn->Set( &p->viewContent );
	else
		rgn->SetEmpty();
}


/*
**	DTSOffView::DTSOffView()
*/
DTSOffView::DTSOffView() :
	offImage()
{
}


/*
**	DTSOffView::~DTSOffView()
*/
DTSOffView::~DTSOffView()
{
	DTSViewPriv * p = priv.p;
	if ( not p )
		return;
	
	GrafPtr port = p->viewPort;
	if ( port )
		{
		::DisposePort( port );
		cacheMap = nullptr;
		}
}


extern CTabHandle	gCTable1;
extern CTabHandle	gCTable8;
extern CTabHandle	gCTable16;
extern CTabHandle	gCTable32;

/*
**	DTSOffView::Init()
*/
void
DTSOffView::Init( int depth )
{
	DTSViewPriv * p = priv.p;
	if ( not p )
		return;
	
	GDHandle * pDevice;
	CTabHandle * pTable;
	switch ( depth )
		{
		case 1:
			pDevice = &gGDevice1;
			pTable  = &gCTable1;
			break;
		
		case 8:
			pDevice = &gGDevice8;
			pTable  = &gCTable8;
			break;
		
		case 16:
			pDevice = &gGDevice16;
			pTable  = &gCTable16;
			break;
		
		case 32:
			pDevice = &gGDevice32;
			pTable  = &gCTable32;
			break;
		
		default:
#ifdef DEBUG_VERSION
			VDebugStr( "DTSOffView only of depth 1, 8, 16, or 32." );
#endif
			SetErrorCode( -1 );
			return;
		}
	
	if ( not *pDevice )
		{
		// allocate and initialize the fake device for this depth
		GDHandle hdev = reinterpret_cast<GDHandle>( ::NewHandleClear( sizeof(GDevice) ) );
		CheckPointer( hdev );
		
		PixMapHandle map = reinterpret_cast<PixMapHandle>( ::NewHandleClear( sizeof(PixMap) ) );
		CheckPointer( map );
		
		ITabHandle itab = reinterpret_cast<ITabHandle>( ::NewHandleClear( sizeof(ITab) ) );
		CheckPointer( itab );
		
		Handle data = ::NewHandle( 0 );
		CheckPointer( data );
		
		Handle mask = ::NewHandle( 0 );
		CheckPointer( mask );
		
		CTabHandle table = *pTable;
		if ( not table )
			{
			table = MyGetCTable( depth );
			CheckPointer( table );
			if ( 8 == depth )
				SyncPaletteTable();
			
			*pTable = table;
			}
		
		assert( hdev );
		assert( map );
		
		*pDevice = hdev;
		GDPtr pdev      = *hdev;
		pdev->gdITable  = itab;
		pdev->gdResPref = 4;
		pdev->gdPMap    = map;
		pdev->gdCCXData = data;
		pdev->gdCCXMask = mask;
		
		PixMapPtr pmap  = *map;
		pmap->hRes      = 0x00480000;
		pmap->vRes      = 0x00480000;
		pmap->pixelSize = depth;
		pmap->pmTable   = table;
		
		if ( depth <= 8 )
			{
			pmap->cmpCount  = 1;
			pmap->cmpSize   = depth;
			}
		else
			{
			pdev->gdType    = directType;
			pmap->pixelType = 0x10;
			pmap->cmpCount  = 3;
			if ( 16 == depth )
				pmap->cmpSize = 5;
			else
				pmap->cmpSize = 8;
			}
		}
	
	GrafPtr savePort;
	::GetPort( &savePort );
	
	GDHandle saveDevice = ::GetGDevice();
	GDHandle hdev = *pDevice;
	::SetGDevice( hdev );
	
	CGrafPtr port = ::CreateNewPort();
	CheckPointer( port );

	::SetGDevice( saveDevice );
	::SetPort( savePort );
	CheckError();
	
	p->InitViewPort( GrafPtr(port), hdev );
}


/*
**	DTSOffView::SetImage()
*/
void
DTSOffView::SetImage( DTSImage * image )
{
	// check for an uninitialized image
	if ( not image
	||   not image->GetBits() )
		{
		VDebugStr( "Warning: Bad DTSImage passed to DTSOffView::SetImage." );
		SetErrorCode( -1 );
		return;
		}
	
	DTSViewPriv * p = priv.p;
	if ( not p )
		return;
	
	CGrafPtr port = reinterpret_cast<CGrafPtr>( p->viewPort );
	PixMapPtr pmap = *::GetPortPixMap( port );
	PixMapPtr omap = (PixMapPtr) DTS2Mac(image);
	
	offImage = image;
	
	image->GetBounds( &p->viewBounds );
	p->viewContent.Set( &p->viewBounds );
	
	pmap->baseAddr = omap->baseAddr;
	pmap->rowBytes = (pmap->rowBytes & ~0x3fff) | omap->rowBytes;
	pmap->bounds   = omap->bounds;
	
	pmap->packType   = omap->packType;
	pmap->packSize   = omap->packSize;
	pmap->hRes       = omap->hRes;
	pmap->vRes       = omap->vRes;
	pmap->pixelType  = omap->pixelType;
	pmap->pixelSize  = omap->pixelSize;
	pmap->cmpCount   = omap->cmpCount;
	pmap->cmpSize    = omap->cmpSize;
#if OLDPIXMAPSTRUCT
	pmap->pmVersion  = omap->pmVersion;
	pmap->planeBytes = omap->planeBytes;
	pmap->pmReserved = omap->pmReserved;
#else
	pmap->pixelFormat= omap->pixelFormat;
	pmap->pmExt 	 = omap->pmExt;
#endif
	
	DTSRect b;
	image->GetBounds( &b );
	
	::SetPortBounds( port, DTS2Mac(&b) );
	RgnHandle rgn = ::NewRgn();
	::RectRgn( rgn, DTS2Mac(&b) );
	::SetPortVisibleRegion( port, rgn );
	::SetPortClipRegion( port, rgn );
	::DisposeRgn( rgn );
}


/*
**	DTSOffView::Set1Pixel()
*/
void
DTSOffView::Set1Pixel( DTSCoord h, DTSCoord v, const DTSColor * color )
{
	// set the gdevice because Color2Index looks at it
	StDrawEnv saveEnv( this );
	
	// convert the color to an index
	int index = ::Color2Index( DTS2Mac(color) );
	
	// set the pixel by index
	Set1Pixel( h, v, index );
}


/*
**	DTSOffView::Set1Pixel()
*/
void
DTSOffView::Set1Pixel( DTSCoord h, DTSCoord v, int index )
{
	DTSViewPriv * p = priv.p;
	if ( not p )
		return;
	
	// get pointer to pix map
	// get pixel size
	uint bitsPerPixel = 1;
	GrafPtr port = p->viewPort;
	
	PixMapPtr map = *::GetPortPixMap( port );
	bitsPerPixel = map->pixelSize;
	
	// cache the base address to avoid the function call
	uchar * addr;
	if ( map == cacheMap )
		addr = cacheAddr;
	else
		{
		addr = reinterpret_cast<uchar *>( ::GetPixBaseAddr( &map ));
		cacheMap  = map;
		cacheAddr = addr;
		}
	
	// get pointer to beginning of row
	addr += ( v - map->bounds.top ) * ( map->rowBytes & 0x7FFF );
	
	// calculate horizontal coordinate in bits from left
	uint horz = ( h - map->bounds.left ) << shiftTable[ bitsPerPixel ];
	
	// offset addr to the word that contains this pixel
	addr += horz >> 5 << 2;
	
	// horz is number of bits below the target pixel
	horz = 32 - bitsPerPixel - (horz & 31);
	
	// calculate the pixel mask
	// ones where the new pixel goes
	uint32_t mask;
	if ( 32 == bitsPerPixel )	// intel doesn't like 32-bit left shifts
		mask = 0xFFFFFFFU << horz;
	else
		mask = ( (1U << bitsPerPixel) - 1U ) << horz;
	
	// insert the pixel
	* (uint32_t *) addr = ((* (uint32_t *) addr) & ~mask) | (index << horz);
}


/*
**	DTSOffView::Get1Pixel()
*/
void
DTSOffView::Get1Pixel( DTSCoord h, DTSCoord v, DTSColor * color ) const
{
	// get the index
	int index = Get1Pixel( h, v );
	
	// convert it to a color
	::Index2Color( index, DTS2Mac(color) );
}


/*
**	DTSOffView::Get1Pixel()
*/
int
DTSOffView::Get1Pixel( DTSCoord h, DTSCoord v ) const
{
	DTSViewPriv * p = priv.p;
	if ( not p )
		return -1;
	
	// get pointer to pix map
	// get pixel size
	uint bitsPerPixel = 1;
	GrafPtr port = p->viewPort;

	PixMapPtr map = *::GetPortPixMap( port );
	bitsPerPixel = map->pixelSize;
	
	// cache the base address to avoid the function call
	uchar * addr;
	if ( map == cacheMap )
		addr = cacheAddr;
	else
		{
		addr = reinterpret_cast<uchar *>( ::GetPixBaseAddr( &map ));
		cacheMap  = map;
		cacheAddr = addr;
		}
	
	// get pointer to beginning of row
	addr += ( v - map->bounds.top ) * ( map->rowBytes & 0x7FFF );
	
	// calculate horizontal coordinate in bits from left
	uint horz = ( h - map->bounds.left ) << shiftTable[ bitsPerPixel ];
	
	// offset addr to the word that contains this pixel
	addr += horz >> 5 << 2;
	
	// return the pixel
	return ( (* (uint32_t *) addr) << (horz & 31) >> (32 - bitsPerPixel) );
}
#endif  // ! AVOID_QUICKDRAW

