/*
**	Draw_mac.cp		dtslib2
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

#include "Memory_dts.h"
#include "Shell_mac.h"
#include "View_mac.h"


/*
**	Entry Routines -- some require QuickDraw, i.e. pre-OS 10.9
**
**	DTSColor;
**	DTSPattern; 			// QD only
**	DTSRect;
**	DTSPolygon;		// totally removed
**	DTSRegion;				// QD only
**	DTSPicture;				// QD only
**	DTSImage;
**
**	MyGetCTable();			// QD only
**	SyncPaletteTable();		// QD only
*/

/*
**	Globals
*/
const DTSColor	DTSColor::black ( 0x0000, 0x0000, 0x0000 );
const DTSColor	DTSColor::white ( 0xFFFF, 0xFFFF, 0xFFFF );
const DTSColor	DTSColor::red   ( 0xFFFF, 0x0000, 0x0000 );
const DTSColor	DTSColor::green ( 0x0000, 0xFFFF, 0x0000 );
const DTSColor	DTSColor::blue  ( 0x0000, 0x0000, 0xFFFF );
const DTSColor	DTSColor::grey25( 0x3FFF, 0x3FFF, 0x3FFF );
const DTSColor	DTSColor::grey50( 0x7FFF, 0x7FFF, 0x7FFF );
const DTSColor	DTSColor::grey75( 0xBFFF, 0xBFFF, 0xBFFF );

// If we're eschewing QD then these guys are file-static
// (although I would dearly love to get rid of them completely);
// otherwise they need to be visible to View_mac.cp also
#ifdef AVOID_QUICKDRAW
static CTabHandle	gCTable1;
static CTabHandle	gCTable8;
static CTabHandle	gCTable16;
static CTabHandle	gCTable32;
#else
CTabHandle			gCTable1;
CTabHandle			gCTable8;
CTabHandle			gCTable16;
CTabHandle			gCTable32;
#endif  // ! AVOID_QUICKDRAW


/*
**	Internal Routines
*/
#ifndef AVOID_QUICKDRAW
static DTSError DrawPictureInImage( DTSImage * image, DTSPicture * picture, int depth );
#endif


/*
**	DTSColor::ChooseDlg()
**
**	put up the color picker dialog
*/
bool
DTSColor::ChooseDlg( const char * prompt )
{
	Str255 pp;
	myctopstr( prompt, pp );
	
	const Point where = { -1, -1 };	// deepest screen
	bool result = ::GetColor( where, pp, DTS2Mac(this), DTS2Mac(this) );
	
#ifndef AVOID_QUICKDRAW
	::InitCursor();
#endif
	
	return result;
}


#ifndef AVOID_QUICKDRAW
/*
**	DTSPattern
**
**	implement this class as quickly and as quietly as possible.
**	all methods call through to the private class
*/
DTSDefineImplementFirmNoCopy(DTSPatternPriv)


/*
**	DTSPatternPriv::DTSPatternPriv()
*/
DTSPatternPriv::DTSPatternPriv() :
	patHdl( nullptr )
{
	Set( 0xFFFFFFFFU );
}


/*
**	DTSPatternPriv::~DTSPatternPriv()
*/
DTSPatternPriv::~DTSPatternPriv()
{
	Reset();
}


/*
**	DTSPattern::SetWhite()
*/
void
DTSPattern::SetWhite()
{
	if ( DTSPatternPriv * p = priv.p )
		p->Set( 0x00000000U );
}


/*
**	DTSPattern::SetGray25()
*/
void
DTSPattern::SetGray25()
{
	if ( DTSPatternPriv * p = priv.p )
		p->Set( 0x88228822U );
}


/*
**	DTSPattern::SetGray50()
*/
void
DTSPattern::SetGray50()
{
	if ( DTSPatternPriv * p = priv.p )
		p->Set( 0xAA55AA55U );
}


/*
**	DTSPattern::SetGray75()
*/
void
DTSPattern::SetGray75()
{
	if ( DTSPatternPriv * p = priv.p )
		p->Set( 0x77DD77DDU );
}


/*
**	DTSPattern::SetBlack()
*/
void
DTSPattern::SetBlack()
{
	if ( DTSPatternPriv * p = priv.p )
		p->Set( 0xFFFFFFFFU );
}


/*
**	DTSPatternPriv::Set()
*/
void
DTSPatternPriv::Set( uint bw )
{
	Reset();
	
	* (uint32_t *) &pattern.pat[0] = bw;
	* (uint32_t *) &pattern.pat[4] = bw;
}


/*
**	DTSPattern::Load()
**
**	load the pattern from the resource file
*/
DTSError
DTSPattern::Load( int id )
{
	DTSPatternPriv * p = priv.p;
	if ( not p )
		return -1;
	
	p->Reset();
	p->patHdl = ::GetPixPat( id );
	
	return ::QDError();
}


/*
**	DTSPatternPriv::Reset()
**
**	Dispose of the color pattern
*/
void
DTSPatternPriv::Reset()
{
	if ( /* PixPatHandle hPat = */ patHdl )
		{
		::DisposePixPat( patHdl );
		patHdl = nullptr;
		}
}
#endif  // ! AVOID_QUICKDRAW


#ifndef AVOID_QUICKDRAW
/*
**	DTSPoint::InRegion()
*/
bool
DTSPoint::InRegion( const DTSRegion * rgn ) const
{
	return rgn ? ::PtInRgn( *DTS2Mac(this), DTS2Mac(rgn) ) : false;
}
#endif  // ! AVOID_QUICKDRAW


/*
**	DTSRect::Set()
*/
void
DTSRect::Set( const DTSPoint& pt1, const DTSPoint& pt2 )
{
	DTSCoord coord1 = pt1.ptV;
	DTSCoord coord2 = pt2.ptV;
	if ( coord1 <= coord2 )
		{
		rectTop    = coord1;
		rectBottom = coord2;
		}
	else
		{
		rectTop    = coord2;
		rectBottom = coord1;
		}
	
	coord1 = pt1.ptH;
	coord2 = pt2.ptH;
	if ( coord1 <= coord2 )
		{
		rectLeft   = coord1;
		rectRight  = coord2;
		}
	else
		{
		rectLeft   = coord2;
		rectRight  = coord1;
		}
}


/*
**	DTSRect::Expand()
*/
void
DTSRect::Expand( DTSCoord h, DTSCoord v )
{
	if ( rectTop > v )
		rectTop = v;
	if ( rectBottom < v )
		rectBottom = v;
	
	if ( rectLeft > h )
		rectLeft = h;
	if ( rectRight < h )
		rectRight = h;
}


/*
**	DTSRect::Union()
*/
void
DTSRect::Union( const DTSRect * other )
{
	if ( not other )
		return;
	
	DTSCoord coord = other->rectTop;
	if ( rectTop > coord )
		rectTop = coord;
	coord = other->rectBottom;
	if ( rectBottom < coord )
		rectBottom = coord;
	
	coord = other->rectLeft;
	if ( rectLeft > coord )
		rectLeft = coord;
	coord = other->rectRight;
	if ( rectRight < coord )
		rectRight = coord;
}


/*
**	DTSRect::Intersect()
*/
void
DTSRect::Intersect( const DTSRect * other )
{
	if ( not other )
		return;
	
	DTSCoord coord = other->rectTop;
	if ( rectTop < coord )
		rectTop = coord;
	coord = other->rectBottom;
	if ( rectBottom > coord )
		rectBottom = coord;
	
	coord = other->rectLeft;
	if ( rectLeft < coord )
		rectLeft = coord;
	coord = other->rectRight;
	if ( rectRight > coord )
		rectRight = coord;
}


#if 0	// polys gone!
/*
**	DTSPolygon
**
**	implement this class as quickly and as quietly as possible
**	all methods call through to the private class
*/
DTSDefineImplementFirm(DTSPolygonPriv)


/*
**	DTSPolygonPriv::DTSPolygonPriv()
*/
DTSPolygonPriv::DTSPolygonPriv()
{
	polyHdl = ::OpenPoly();
	::ClosePoly();
	polyBounds.Set( 0x8000, 0x8000, 0x8000, 0x8000 );
	
	SetErrorCode( ::QDError() );
}


/*
**	DTSPolygonPriv::DTSPolygonPriv()
*/
DTSPolygonPriv::DTSPolygonPriv( const DTSPolygonPriv & rhs )
{
	polyHdl = rhs.polyHdl;
	if ( noErr != ::HandToHand( reinterpret_cast<Handle *>( &polyHdl ) ) )
		polyHdl = nullptr;
	polyBounds = rhs.polyBounds;
}


/*
**	DTSPolygonPriv::DTSPolygonPriv()
*/
DTSPolygonPriv &
DTSPolygonPriv::operator=( const DTSPolygonPriv & rhs )
{
	if ( &rhs != this )
		{
		polyHdl = rhs.polyHdl;
		if ( noErr != ::HandToHand( reinterpret_cast<Handle *>( &polyHdl ) ) )
			polyHdl = nullptr;
		polyBounds = rhs.polyBounds;
		}
		
	return *this;
}


/*
**	DTSPolygonPriv::~DTSPolygonPriv()
*/
DTSPolygonPriv::~DTSPolygonPriv()
{
	::KillPoly( polyHdl );
}


/*
**	DTSPolygon::Add()
**
**	Add a point to the macintosh polygon.
*/
void
DTSPolygon::Add( DTSCoord h, DTSCoord v )
{
	DTSPolygonPriv * p = priv.p;
	if ( not p )
		return;
	
	if ( not p->polyHdl )
		return;
	
	::Size newSize;
	newSize = ::GetHandleSize( Handle(p->polyHdl) ) + sizeof(Point);
	::SetHandleSize( Handle(p->polyHdl), newSize );
	
	SetErrorCode( ::MemError() );
	CheckError();
	
	PolyPtr poly = *p->polyHdl;
	poly->polySize = short(newSize);
	
	if ( short( 0x8000 ) == p->polyBounds.rectTop )
		p->polyBounds.Set( h, v, h, v );
	else
		p->polyBounds.Expand( h, v );
	
	poly->polyBBox = * reinterpret_cast<Rect *>( &p->polyBounds );
	
	Point * ppt = reinterpret_cast<Point *>(
					reinterpret_cast<char *>( poly ) + newSize - sizeof(Point) );
	ppt->v = v;		// endian??
	ppt->h = h;
}


/*
**	DTSPolygon::Offset()
**
**	Offset the bounding box of the macintosh polygon.
*/
void
DTSPolygon::Offset( DTSCoord h, DTSCoord v )
{
	DTSPolygonPriv * p = priv.p;
	if ( not p )
		return;
	
	if ( not p->polyHdl )
		return;
	
	::OffsetPoly( p->polyHdl, h, v );
	p->polyBounds.Offset( h, v );
}


/*
**	DTSPolygon::GetBounds()
**
**	Return the bounding box of the polygon.
*/
void
DTSPolygon::GetBounds( DTSRect * bounds ) const
{
	if ( not bounds )
		return;
	
	if ( DTSPolygonPriv * p = priv.p )
		*bounds = p->polyBounds;
	else
		bounds->Set( 0x8000, 0x8000, 0x8000, 0x8000 );
}


/*
**	DTSPolygon::Reset()
**
**	Reset the polygon to be empty.
*/
void
DTSPolygon::Reset()
{
	DTSPolygonPriv * p = priv.p;
	if ( not p )
		return;
	
	::KillPoly( p->polyHdl );
	
	p->polyHdl = ::OpenPoly();
	::ClosePoly();
	p->polyBounds.Set( 0x8000, 0x8000, 0x8000, 0x8000 );
	
	SetErrorCode( ::QDError() );
}
#endif  // 0	-- no more polygons


#ifndef AVOID_QUICKDRAW
/*
**	DTSRegion
**
**	implement this class as quickly and as quietly as possible
*/
DTSDefineImplementFirm(DTSRegionPriv)


/*
**	DTSRegionPriv::DTSRegionPriv()
*/
DTSRegionPriv::DTSRegionPriv()
{
	rgnHdl = ::NewRgn();
	SetErrorCode( ::QDError() );
	CheckError();
}


/*
**	DTSRegionPriv::DTSRegionPriv()
*/
DTSRegionPriv::DTSRegionPriv( const DTSRegionPriv & rhs )
{
	DTSError result = noErr;
	rgnHdl = ::NewRgn();
	if ( rgnHdl )
		{
		::CopyRgn( rhs.rgnHdl, rgnHdl );
		result = ::QDError();
		}
	else
		result = memFullErr;
	
	SetErrorCode( result );
}


/*
**	DTSRegionPriv::~DTSRegionPriv()
*/
DTSRegionPriv::~DTSRegionPriv()
{
	::DisposeRgn( rgnHdl );
}


/*
**	DTSRegionPriv::operator=()
*/
DTSRegionPriv &
DTSRegionPriv::operator=( const DTSRegionPriv & rhs )
{
	if ( this != &rhs )
		{
		::CopyRgn( rhs.rgnHdl, rgnHdl );
		SetErrorCode( ::QDError() );
		}
	
	return *this;
}


/*
**	DTSRegion::SetEmpty()
*/
void
DTSRegion::SetEmpty()
{
	DTSRegionPriv * p = priv.p;
	if ( not p )
		return;
	
	if ( p->rgnHdl )
		{
		::SetEmptyRgn( p->rgnHdl );
		SetErrorCode( ::QDError() );
		}
}


/*
**	DTSRegion::Set()
*/
void
DTSRegion::Set( const DTSRect * rect )
{
	if ( not rect )
		return;
	
	DTSRegionPriv * p = priv.p;
	if ( not p )
		return;
	
	if ( p->rgnHdl )
		{
		::RectRgn( p->rgnHdl, DTS2Mac(rect) );
		SetErrorCode( ::QDError() );
		}
}


/*
**	DTSRegion::Set()
*/
void
DTSRegion::Set( const DTSOval * oval )
{
	if ( not oval )
		return;
	
	DTSRegionPriv * p = priv.p;
	if ( not p )
		return;
	
	if ( p->rgnHdl )
		{
		::OpenRgn();
		::FrameOval( DTS2Mac(oval) );
		::CloseRgn( p->rgnHdl );
		SetErrorCode( ::QDError() );
		}
}


/*
**	DTSRegion::Set()
*/
void
DTSRegion::Set( const DTSRound * round )
{
	if ( not round )
		return;
	
	DTSRegionPriv * p = priv.p;
	if ( not p )
		return;
	
	if ( p->rgnHdl )
		{
		::OpenRgn();
		::FrameRoundRect( DTS2Mac(round), round->roundWidth, round->roundHeight );
		::CloseRgn( p->rgnHdl );
		SetErrorCode( ::QDError() );
		}
}


#if 0  // no more polygons
/*
**	DTSRegion::Set()
*/
void
DTSRegion::Set( const DTSPolygon * poly )
{
	if ( not poly )
		return;
	
	DTSRegionPriv * p = priv.p;
	if ( not p )
		return;
	
	if ( p->rgnHdl )
		{
		::OpenRgn();
		::FramePoly( DTS2Mac(poly) );
		::CloseRgn( p->rgnHdl );
		SetErrorCode( ::QDError() );
		}
}
#endif  // 0


/*
**	DTSRegion::Set()
*/
void
DTSRegion::Set( const DTSRegion * other )
{
	if ( not other )
		return;
	
	DTSRegionPriv * p = priv.p;
	if ( not p )
		return;
	
	if ( p->rgnHdl )
		{
		::CopyRgn( DTS2Mac(other), p->rgnHdl );
		SetErrorCode( ::QDError() );
		}
}


/*
**	DTSRegion::Offset()
*/
void
DTSRegion::Offset( DTSCoord dh, DTSCoord dv )
{
	if ( DTSRegionPriv * p = priv.p )
		{
		if ( p->rgnHdl )
			::OffsetRgn( p->rgnHdl, dh, dv );
		}
}


/*
**	DTSRegion::Move()
*/
void
DTSRegion::Move( DTSCoord horz, DTSCoord vert )
{
	if ( DTSRegionPriv * p = priv.p )
		{
		if ( p->rgnHdl )
			{
			DTSRect box;
			GetBounds( &box );
			::OffsetRgn( p->rgnHdl, horz - box.rectLeft, vert - box.rectTop );
			}
		}
}


/*
**	DTSRegion::Union()
*/
void
DTSRegion::Union( const DTSRegion * other )
{
	if ( not other )
		return;
	
	if ( DTSRegionPriv * p = priv.p )
		{
		if ( p->rgnHdl )
			{
			::UnionRgn( p->rgnHdl, DTS2Mac(other), p->rgnHdl );
			SetErrorCode( ::QDError() );
			}
		}
}


/*
**	DTSRegion::Intersect()
*/
void
DTSRegion::Intersect( const DTSRegion * other )
{
	if ( not other )
		return;
	
	if ( DTSRegionPriv * p = priv.p )
		{
		if ( p->rgnHdl )
			{
			::SectRgn( p->rgnHdl, DTS2Mac(other), p->rgnHdl );
			SetErrorCode( ::QDError() );
			}
		}
}


/*
**	DTSRegion::Subtract()
*/
void
DTSRegion::Subtract( const DTSRegion * other )
{
	if ( not other )
		return;
	
	if ( DTSRegionPriv * p = priv.p )
		{
		if ( p->rgnHdl )
			{
			::DiffRgn( p->rgnHdl, DTS2Mac(other), p->rgnHdl );
			SetErrorCode( ::QDError() );
			}
		}
}


/*
**	DTSRegion::Xor()
*/
void
DTSRegion::Xor( const DTSRegion * other )
{
	if ( not other )
		return;
	
	if ( DTSRegionPriv * p = priv.p )
		{
		if ( p->rgnHdl )
			{
			::XorRgn( p->rgnHdl, DTS2Mac(other), p->rgnHdl );
			SetErrorCode( ::QDError() );
			}
		}
}


/*
**	DTSRegion::GetBounds()
*/
void
DTSRegion::GetBounds( DTSRect * bounds ) const
{
	if ( not bounds )
		return;
	
	DTSRegionPriv * p = priv.p;
	RgnHandle rgn = p ? p->rgnHdl : nullptr;
	
	if ( rgn )
		::GetRegionBounds( rgn, DTS2Mac(bounds) );
	else
		bounds->Set( 0x8000, 0x8000, 0x8000, 0x8000 );
}


/*
**	DTSPicture
**
**	Implement this class as quickly and as quietly as possible
*/
DTSDefineImplementFirm(DTSPicturePriv)


/*
**	DTSPicturePriv::DTSPicturePriv()
*/
DTSPicturePriv::DTSPicturePriv() :
	picHdl( nullptr ),
	picResource( false )
{
}


/*
**	DTSPicturePriv::DTSPicturePriv()
*/
DTSPicturePriv::DTSPicturePriv( const DTSPicturePriv & rhs )
{
	PicHandle hdl = rhs.picHdl;
	DTSError result = ::HandToHand( (Handle *) &hdl );
	if ( noErr == result )
		picHdl = hdl;
	else
		picHdl = nullptr;
	picResource = false;
}


/*
**	DTSPicturePriv::operator=()
*/
DTSPicturePriv &
DTSPicturePriv::operator=( const DTSPicturePriv & rhs )
{
	if ( this != &rhs )
		{
		PicHandle hdl = rhs.picHdl;
		DTSError result = ::HandToHand( (Handle *) &hdl );
		if ( noErr == result )
			picHdl = hdl;
		else
			picHdl = nullptr;
		picResource = false;
		}
	return *this;
}


/*
**	DTSPicturePriv::~DTSPicturePriv()
*/
DTSPicturePriv::~DTSPicturePriv()
{
	Reset();
}


/*
**	DTSPicture::Load()
**
**	load from a 'PICT' resource with the given ID
*/
DTSError
DTSPicture::Load( int id )
{
	DTSPicturePriv * p = priv.p;
	if ( not p )
		return -1;
	
	Reset();
	
	PicHandle pict = ::GetPicture( id );
	SetErrorCode( ::ResError() );
	if ( not pict )
		return resNotFound;
	
	::HNoPurge( (Handle) pict );
	p->picHdl      = pict;
	p->picResource = true;
	
	return noErr;
}


/*
**	DTSPicture::GetFromScrap()
**
**	load from a 'PICT' on the scrap (if there is one)
*/
DTSError
DTSPicture::GetFromScrap()
{
	DTSPicturePriv * p = priv.p;
	if ( not p )
		return -1;
	
	// reset the picture
	Reset();
	
	// get the size of the picture from the scrap
	ScrapRef scrap;
	::Size sz = 0;
	
	// check for an error and set the error code
	OSStatus result = ::GetCurrentScrap( &scrap );
	if ( noErr == result )
		result = ::GetScrapFlavorSize( scrap, kScrapFlavorTypePicture, &sz );
	else
		return result;
	
	// allocate a picture handle
	Handle pict = ::NewHandle( sz );
	if ( not pict )
		return memFullErr;
	
	// load the picture from the scrap
//	::HLock( pict );
	result = ::GetScrapFlavorData( scrap, kScrapFlavorTypePicture, &sz, *pict );
//	::HUnlock( pict );
	
	if ( result != noErr )
		return result;
	
	p->picHdl      = PicHandle( pict );
	p->picResource = false;
	
	return noErr;
}


/*
**	DTSPicture::Reset()
*/
void
DTSPicture::Reset()
{
	if ( DTSPicturePriv * p = priv.p )
		p->Reset();
}


/*
**	DTSPicturePriv::Reset()
*/
void
DTSPicturePriv::Reset()
{
	PicHandle pict = picHdl;
	if ( pict )
		{
		if ( picResource )
			::HPurge( (Handle) pict );
		else
			::KillPicture( pict );
		
		picHdl      = nullptr;
		picResource = false;
		}
}


/*
**	DTSPicture::Set()
**
**	create a picture from the view (via CopyBits)
*/
void
DTSPicture::Set( DTSView * view, const DTSRect * box )
{
	if ( not view || not box )
		return;
	
	DTSPicturePriv * p = priv.p;
	if ( not p )
		return;
	
	StDrawEnv saveEnv( view );
	
	PicHandle pict = ::OpenPicture( DTS2Mac(box) );
	if ( pict )
		view->Draw( view, kImageCopyQuality, box, box );
	::ClosePicture();
	
	p->Reset();
	if ( pict )
		{
		p->picHdl = pict;
		p->picResource = false;
		}
}


/*
**	DTSPicture::CopyToScrap()
**
**	copy the picture to the scrap
*/
DTSError
DTSPicture::CopyToScrap() const
{
	DTSPicturePriv * p = priv.p;
	if ( not p )
		return -1;
	
	Handle pict = (Handle) p->picHdl;
	if ( not pict )
		return -1;
	
//	SInt8 state = ::HGetState( pict );
//	::HLock( pict );
	
	OSStatus result = ::ClearCurrentScrap();
	ScrapRef scrap;
	if ( noErr == result )
		result = ::GetCurrentScrap( &scrap );
	if ( noErr == result )
		result = ::PutScrapFlavor( scrap, kScrapFlavorTypePicture, kScrapFlavorMaskNone,
					GetHandleSize( pict ), *pict );
	
//	::HSetState( pict, state );
	
	return result;
}


/*
**	DTSPicture::GetBounds()
**
**	return the bounding box
*/
void
DTSPicture::GetBounds( DTSRect * bounds ) const
{
	if ( not bounds )
		return;
	
	DTSPicturePriv * p = priv.p;
	PicHandle pict = p ? p->picHdl : nullptr;
	
	if ( pict )
		QDGetPictureBounds( pict, DTS2Mac(bounds) );
	else
		bounds->Set( 0x8000, 0x8000, 0x8000, 0x8000 );
}
#endif  // ! AVOID_QUICKDRAW


/*
**	DTSImage
**
**	implement this class as quickly and as quietly as possible
*/
DTSDefineImplementFirmNoCopy(DTSImagePriv)


/*
**	DTSImagePriv::DTSImagePriv()
*/
DTSImagePriv::DTSImagePriv() :
	imageAllocated( ipAllocNone ),
	imageRowBytes( 0 )
{
	imageMap.baseAddr = nullptr;
}


/*
**	DTSImagePriv::~DTSImagePriv()
*/
DTSImagePriv::~DTSImagePriv()
{
	DisposeBits();
}


#ifdef AVOID_QUICKDRAW
static CTabHandle MyGetCTable( int depth );
#else
// otherwise, it's declared in View_mac.h, and used in View_mac.cp as well as here
#endif


/*
**	DTSImage::Init()
*/
DTSError
DTSImage::Init( void * bits, DTSCoord width, DTSCoord height, int depth )
{
	DTSImagePriv * p = priv.p;
	if ( not p )
		return -1;
	
	if ( 1 == depth )
		{
		if ( not gCTable1 )
			{
			gCTable1 = MyGetCTable( 1 );
			if ( not gCTable1 )
				return memFullErr;
			}
		
		p->imageRowBytes        = ( (width + 0x0F) & ~0x0F ) >> 3;
		p->imageMap.rowBytes    = short(p->imageRowBytes) | 0x8000;
		p->imageMap.pixelType   = 0;
		p->imageMap.pixelSize   = 1;
		p->imageMap.cmpCount    = 1;
		p->imageMap.cmpSize     = 1;
		p->imageMap.pmTable     = gCTable1;
		}
	else
	if ( 8 == depth )
		{
		if ( not gCTable8 )
			{
			gCTable8 = MyGetCTable( 8 );
			if ( not gCTable8 )
				return memFullErr;
#ifndef AVOID_QUICKDRAW
			SyncPaletteTable();
#endif
			}
		
		p->imageRowBytes      = (width + 0x03) & ~0x03;
		p->imageMap.rowBytes  = short(p->imageRowBytes) | 0x8000;
		p->imageMap.pixelType = 0;
		p->imageMap.pixelSize = 8;
		p->imageMap.cmpCount  = 1;
		p->imageMap.cmpSize   = 8;
		p->imageMap.pmTable   = gCTable8;
		}
	else
	if ( 16 == depth )
		{
		if ( not gCTable16 )
			{
			gCTable16 = MyGetCTable( 16 );
			if ( not gCTable16 )
				return memFullErr;
			}
		
		p->imageRowBytes      = (( width + 0x01) & ~0x01 ) << 1;
		p->imageMap.rowBytes  = short(p->imageRowBytes) | 0x8000;
		p->imageMap.pixelType = 0x10;
		p->imageMap.pixelSize = 16;
		p->imageMap.cmpCount  = 3;
		p->imageMap.cmpSize   = 5;
		p->imageMap.pmTable   = gCTable16;
		}
	else
	if ( 32 == depth )
		{
		if ( not gCTable32 )
			{
			gCTable32 = MyGetCTable( 32 );
			if ( not gCTable32 )
				return memFullErr;
			}
		
		p->imageRowBytes      = width << 2;
		p->imageMap.rowBytes  = short(p->imageRowBytes) | 0x8000;
		p->imageMap.pixelType = 0x10;
		p->imageMap.pixelSize = 32;
		p->imageMap.cmpCount  = 3;
		p->imageMap.cmpSize   = 8;
		p->imageMap.pmTable   = gCTable32;
		}
	else
		{
		p->imageMap.baseAddr = nullptr;
#ifdef DEBUG_VERSION
		VDebugStr( "DTSImage only of depth 1, 8, 16, or 32." );
#endif
		return -1;
		}
	
	// common initialization
	p->imageMap.baseAddr      = static_cast<char *>( bits );
	p->imageMap.bounds.top    = 0;
	p->imageMap.bounds.left   = 0;
	p->imageMap.bounds.bottom = height;
	p->imageMap.bounds.right  = width;
	p->imageMap.pmVersion     = 0;
	p->imageMap.packType      = 0;
	p->imageMap.packSize      = 0;
	p->imageMap.hRes          = 0x00480000;
	p->imageMap.vRes          = 0x00480000;
#if OLDPIXMAPSTRUCT
	p->imageMap.planeBytes    = 0;
	p->imageMap.pmReserved    = 0;
#else
	p->imageMap.pixelFormat   = 0;
	p->imageMap.pmExt         = 0;
#endif
	
	return noErr;
}


/*
**	DTSImage::AllocateBits()
*/
DTSError
DTSImage::AllocateBits()
{
	if ( DTSImagePriv * p = priv.p )
		return p->AllocateBits();
	
	return -1;
}


/*
**	DTSImagePriv::AllocateBits()
*/
DTSError
DTSImagePriv::AllocateBits()
{
	// allocate memory from the c++ heap
	// or from the mac's heap
	size_t sz = imageMap.bounds.bottom * imageRowBytes;
	char * bits = NEW_TAG("DTSImageBits") char[ sz ];
	AllocKind alloc = ipAllocNew;
	
#if 0	// if we couldn't get it from operator new(), we'll never get it from NewPtr()
	if ( not bits )
		{
		bits = ::NewPtr( sz );
		alloc = ipAllocPtr;
		}
#endif  // 0
	
	// bail if we failed
	if ( not bits )
		return memFullErr;
	
	// dispose of any current bits
	DisposeBits();
	
	// save the new bits and flag
	imageMap.baseAddr = bits;
	imageAllocated    = alloc;
	
	return noErr;
}


/*
**	DTSImage::DisposeBits()
*/
void
DTSImage::DisposeBits()
{
	if ( DTSImagePriv * p = priv.p )
		p->DisposeBits();
}


/*
**	DTSImagePriv::DisposeBits()
*/
void
DTSImagePriv::DisposeBits()
{
	// if we allocated the bits then free the memory
	// from the correct place
	switch ( imageAllocated )
		{
		case ipAllocUnknown:
			// have no idea where the bits pointer came from
			break;
		
		case ipAllocNew:
			delete[] imageMap.baseAddr;
			break;
		
#if 0	// disabled; see above
		case ipAllocPtr:
			::DisposePtr( imageMap.baseAddr );
			break;
#endif  // 0
		
		case ipAllocNone:
			// no bits, nothing to do
			break;
		
		default:
#ifdef DEBUG_VERSION
			VDebugStr( "Bad allocation flag in DTSImage::DisposeBits" );
#endif
			break;
		}
	
	// reset fields
	imageMap.baseAddr = nullptr;
	imageAllocated    = ipAllocNone;
}


#ifndef AVOID_QUICKDRAW
/*
**	DTSImage::Load()
**	from a 'PICT' resource
*/
DTSError
DTSImage::Load( int depth, int id )
{
	DTSPicture pict;
	DTSError result = pict.Load( id );
	if ( result != noErr )
		return result;
	
	result = DrawPictureInImage( this, &pict, depth );
	return result;
}


/*
**	DTSImage::GetFromScrap()
**	from a PICT on the scrap
*/
DTSError
DTSImage::GetFromScrap( int depth )
{
	DTSPicture pict;
	DTSError result = pict.GetFromScrap();
	if ( result != noErr )
		return result;
	
	result = DrawPictureInImage( this, &pict, depth );
	return result;
}
#endif  // ! AVOID_QUICKDRAW


/*
**	DTSImage::SetBits()
**
**	set the pointer to the bits
**	If you use this, you must arrange to dispose the bits yourself when necessary;
**	neither the destructor nor DisposeBits() will do it for you automatically.
*/
void
DTSImage::SetBits( void * bits )
{
	if ( DTSImagePriv * p = priv.p )
		{
		p->imageMap.baseAddr = static_cast<char *>( bits );
		p->imageAllocated    = DTSImagePriv::ipAllocUnknown;
		}
}


/*
**	DTSImage::GetBits()
**
**	return the pointer to the bits
*/
void *
DTSImage::GetBits() const
{
	DTSImagePriv * p = priv.p;
	return p ? p->imageMap.baseAddr : nullptr;
}


/*
**	DTSImage::GetDepth()
**
**	return the number of bits per pixel
*/
int
DTSImage::GetDepth() const
{
	DTSImagePriv * p = priv.p;
	return p ? p->imageMap.pixelSize : -1;
}


/*
**	DTSImage::GetBounds()
**
**	return the bounding box
*/
void
DTSImage::GetBounds( DTSRect * bounds ) const
{
	if ( not bounds )
		return;
	
	if ( DTSImagePriv * p = priv.p )
		*DTS2Mac(bounds) = p->imageMap.bounds;
	else
		bounds->Set( 0x8000, 0x8000, 0x8000, 0x8000 );
}


/*
**	DTSImage::GetRowBytes()
**
**	return the number of bytes per row
*/
int
DTSImage::GetRowBytes() const
{
	DTSImagePriv * p = priv.p;
	return p ? p->imageRowBytes : -1;
}


#ifndef AVOID_QUICKDRAW
/*
**	PrivImageView
**	helper for drawing a PICT into an OffView
*/
class PrivImageView : public DTSOffView
{
public:
	DTSPicture * pivPict;
	virtual void	DoDraw()
		{
		DTSRect viewBounds;
		GetBounds( &viewBounds );
		Erase( &viewBounds ); 
		Draw( pivPict, &viewBounds );
		}
};


/*
**	DrawPictureInImage()
**
**	draw a picture in an image
*/
DTSError
DrawPictureInImage( DTSImage * image, DTSPicture * pict, int depth )
{
	if ( not image || not pict )
		return -1;
	
	PrivImageView view;
	DTSViewPriv * p = view.priv.p;
	if ( not p )
		return -1;
	
	view.pivPict = pict;
	view.Init( depth );
	view.Move( 0, 0 );
	DTSRect bounds;
	pict->GetBounds( &bounds );
	bounds.Move( 0, 0 );
	view.Size( bounds.rectRight, bounds.rectBottom );
	image->Init( nullptr, bounds.rectRight, bounds.rectBottom, depth );
	DTSError result = image->AllocateBits();
	if ( result != noErr )
		return result;
	
	view.SetImage( image );
	view.Show();
	
	return noErr;
}
#endif  // AVOID_QUICKDRAW


# if MAC_OS_X_VERSION_MAX_ALLOWED > 1060
// hmm. In the non-QD case, this is only used for depths 1, 8. Can we fake it?
extern "C" {
	CTabHandle	GetCTable( short ctID );
};
# endif  // 10.7+


/*
**	MyGetCTable()
*/
CTabHandle
MyGetCTable( int depth )
{
	CTabHandle htable;
	
	if ( depth >= 16 )
		{
		htable = (CTabHandle) ::NewHandle( 8 );
		if ( htable )
			{
			CTabPtr ptable = *htable;
			if ( 16 == depth )
				ptable->ctSeed = 16;
			else
				ptable->ctSeed = 24;	// the 32 bit color table has
										// a seed of 24. go figure.
			ptable->ctFlags =  0;
			ptable->ctSize  = -1;
			}
		}
	else
		htable = ::GetCTable( depth );
	
	return htable;
}


#ifndef AVOID_QUICKDRAW
/*
**	SyncPaletteTable()
**
**	keep the 8 bit color table and the palette in sync
*/
void
SyncPaletteTable()
{
	// make sure the 8 bit color table is loaded
	CTabHandle hTable = gCTable8;
	if ( not hTable )
		{
		hTable = MyGetCTable( 8 );
		if ( not hTable )
			return;
		gCTable8 = hTable;
		}
	
	// restore the color table if there is no palette
	if ( (*hTable)->ctSeed != 8 )
		{
		CTabHandle newTable = ::GetCTable( 8 );
		CheckPointer( newTable );
		assert( newTable );
		
		::Size sz = ::GetHandleSize( (Handle) newTable );
		memcpy( *hTable, *newTable, sz );
		::DisposeCTable( newTable );
		}
}
#endif  // AVOID_QUICKDRAW

