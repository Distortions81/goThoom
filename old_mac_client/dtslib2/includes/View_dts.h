/*
**	View_dts.h		dtslib2
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

#ifndef View_dts_h
#define View_dts_h

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "Local_dts.h"
#include "Platform_dts.h"

// GCC 4.x (>4.0.1) emits many warnings about this file if -Wconversion is on
#if defined( __GNUC__ ) && ( __GNUC__ > 4 || ( __GNUC__ == 4 && __GNUC_MINOR__ > 0 ) )
# define PICKYPICKY		1
#endif


/*
	I conditionalized out a bunch of functions that cannot be implemented
	as-is in MacOSX 10.4+, at least not without too many deprecation warnings.
	Gradually, we should start providing non-deprecated alternates.
*/
// eschew Quickdraw on non-Mac or post-10.8 builds
#if ! defined( MAC_OS_X_VERSION_MAX_ALLOWED ) || MAC_OS_X_VERSION_MAX_ALLOWED >= 1070
# define AVOID_QUICKDRAW	1
#else
# undef AVOID_QUICKDRAW
#endif


/*
**	Define key equivalents
*/
enum
{
	kHomeKey			= 0x01,
	kEnterKey			= 0x03,
	kEndKey				= 0x04,
	kHelpKey			= 0x05,
	kBackspaceKey		= 0x08,
	kDeleteKey			= 0x08,
	kTabKey				= 0x09,
	kPageUpKey			= 0x0B,
	kPageDownKey		= 0x0C,
	kReturnKey			= 0x0D,
	kEscapeKey			= 0x1B,
	kLeftArrowKey		= 0x1C,
	kRightArrowKey		= 0x1D,
	kUpArrowKey			= 0x1E,
	kDownArrowKey		= 0x1F,
	kRightDeleteKey		= 0x7F,

	// synthetic character codes
	kMenuModKey			= 0x0100,
	kOptionModKey		= 0x0101,
	kShiftModKey		= 0x0102,
	kControlModKey		= 0x0103,
	kCapsLockModKey	 	= 0x0104,
	kF1Key				= 0x0105,
	kF2Key				= 0x0106,
	kF3Key				= 0x0107,
	kF4Key				= 0x0108,
	kF5Key				= 0x0109,
	kF6Key				= 0x010A,
	kF7Key				= 0x010B,
	kF8Key				= 0x010C,
	kF9Key				= 0x010D,
	kF10Key				= 0x010E,
	kF11Key				= 0x010F,
	kF12Key				= 0x0110,
	kF13Key				= 0x0111,
	kF14Key				= 0x0112,
	kF15Key				= 0x0113,
	kF16Key				= 0x0114
};


/*
**	DTSColor class
*/
#pragma pack( push, 2 )

class DTSColor
{
public:
	// order and size depends on implementation
	// Currently & intentionally matches Mac QuickDraw's RGBColor
	// (obsolete as that may be...)
	RGBComp			rgbRed;
	RGBComp			rgbGreen;
	RGBComp			rgbBlue;
	
	// constructor
				DTSColor() {}
	
	// copy constructor
				DTSColor( const DTSColor &rhs ) :
					rgbRed  ( rhs.rgbRed   ),
					rgbGreen( rhs.rgbGreen ),
					rgbBlue ( rhs.rgbBlue  )
					{}
	
	// assignment
	DTSColor&	operator=( const DTSColor& rhs )
		{
		rgbRed   = rhs.rgbRed;
		rgbGreen = rhs.rgbGreen;
		rgbBlue  = rhs.rgbBlue;
		return *this;
		}
	
	// component constructor
				DTSColor( RGBComp r, RGBComp g, RGBComp b ) :
					rgbRed( r ), rgbGreen( g ), rgbBlue( b ) {}
	
	// set the components
	void		Set( RGBComp r, RGBComp g, RGBComp b )
					{ rgbRed = r; rgbGreen = g; rgbBlue = b; }
	
	// common colors
	void		SetBlack()  { Set( 0x0000, 0x0000, 0x0000 ); }
	void		SetWhite()  { Set( 0xFFFF, 0xFFFF, 0xFFFF ); }
	void		SetGrey25() { Set( 0x3FFF, 0x3FFF, 0x3FFF ); }
	void		SetGrey50() { Set( 0x7FFF, 0x7FFF, 0x7FFF ); }
	void		SetGrey75() { Set( 0xBFFF, 0xBFFF, 0xBFFF ); }
	void		SetRed()    { Set( 0xFFFF, 0x0000, 0x0000 ); }
	void		SetGreen()  { Set( 0x0000, 0xFFFF, 0x0000 ); }
	void		SetBlue()   { Set( 0x0000, 0x0000, 0xFFFF ); }
	
	// common colors redux
	static const DTSColor white;
	static const DTSColor black;
	static const DTSColor red;
	static const DTSColor green;
	static const DTSColor blue;
	static const DTSColor grey25;
	static const DTSColor grey50;
	static const DTSColor grey75;
	
	// interface
	bool		ChooseDlg( const char * prompt );
};


// patterns are QD-centric
#ifndef AVOID_QUICKDRAW
/*
**	DTSPattern class
**	Patterns are assumed to be black and white.
**	Size is implementation dependent.
*/
class DTSPatternPriv;
class DTSPattern
{
public:
				DTSImplementNoCopy<DTSPatternPriv> priv;
	
	// set the pattern to a standard
	void		SetWhite();
	void		SetGray25();
	void		SetGray50();
	void		SetGray75();
	void		SetBlack();
	
	// load from resource file
	DTSError	Load( int id );
};
#endif  // ! AVOID_QUICKDRAW


/*
**	DTSPoint class
*/
class DTSPoint
{
public:
	// order is implementation specific
	// Currently identical to a QD Point.
	DTSCoord		ptV;
	DTSCoord		ptH;
	
	// constructor
				DTSPoint() {}
	
	// copy constructor
				DTSPoint( const DTSPoint &rhs ) :
					ptV( rhs.ptV ),
					ptH( rhs.ptH )
					{}
	
	// assignment
	DTSPoint&	operator=( const DTSPoint& rhs )
					{
					ptV = rhs.ptV;
					ptH = rhs.ptH;
					return *this;
					}
	
	// coordinate constructor
				DTSPoint( DTSCoord h, DTSCoord v ) :
					ptV( v ), ptH( h )
					{}
	
	// set the point
	void		Set( DTSCoord h, DTSCoord v )
					{ ptV = v; ptH = h; }
	
	// offset the point
	void		Offset( DTSCoord dh, DTSCoord dv )
					{
#if PICKYPICKY
					ptV = DTSCoord( ptV + dv );
					ptH = DTSCoord( ptH + dh );
#else
					ptV += dv;
					ptH += dh;
#endif
					}
	
	// test if in a rectangle
	inline bool	InRect( const DTSRect * box ) const;
	
#if ! defined( AVOID_QUICKDRAW ) && ! defined( DTS_XCODE )
	// test if in a region
	bool		InRegion( const DTSRegion * rgn ) const;
#endif
	
	// test if equal
	bool		IsEqualTo( const DTSPoint& otherPt ) const
					{ return ptV == otherPt.ptV && ptH == otherPt.ptH; }
};


/*
**	DTSRect class
*/
class DTSRect
{
public:
	// order is implementation dependent
	// Currently identical to a QD Rect.
	DTSCoord		rectTop;
	DTSCoord		rectLeft;
	DTSCoord		rectBottom;
	DTSCoord		rectRight;
	
	// constructor
				DTSRect() {}
	
	// copy constructor
				DTSRect( const DTSRect& rhs ) :
					rectTop( rhs.rectTop ),
					rectLeft( rhs.rectLeft ),
					rectBottom( rhs.rectBottom ),
					rectRight( rhs.rectRight )
					{}
	
	// coordinate constructor
				DTSRect( DTSCoord l, DTSCoord t, DTSCoord r, DTSCoord b ) :
					rectTop( t ),
					rectLeft( l ),
					rectBottom( b ),
					rectRight( r )
					{}
	
	// assignment operator
	DTSRect&	operator=( const DTSRect& rhs )
					{
					if ( &rhs != this )
						{
						rectTop    = rhs.rectTop;
						rectLeft   = rhs.rectLeft;
						rectBottom = rhs.rectBottom;
						rectRight  = rhs.rectRight;
						}
					return *this;
					}
	
	// set the coordinates
	void		Set( const DTSPoint& pt1, const DTSPoint& pt2 );
	
	void		Set( DTSCoord left, DTSCoord top, DTSCoord right, DTSCoord bottom )
					{ rectTop = top; rectLeft = left; rectRight = right; rectBottom = bottom; }
	
	// set the height and width
	void		Size( DTSCoord width, DTSCoord height )
					{ rectBottom = DTSCoord( rectTop + height ); rectRight = DTSCoord( rectLeft + width ); }
	
	// offset the coordinates
	void		Offset( DTSCoord dh, DTSCoord dv )
					{
#if PICKYPICKY
					rectTop    = DTSCoord( rectTop    + dv );
					rectLeft   = DTSCoord( rectLeft   + dh );
					rectBottom = DTSCoord( rectBottom + dv );
					rectRight  = DTSCoord( rectRight  + dh );
#else
					rectTop    += dv;
				   	rectLeft   += dh;
				   	rectBottom += dv;
				   	rectRight  += dh;
#endif
					}
	
	// inset the coordinates
	void		Inset( DTSCoord dh, DTSCoord dv )
					{
#if PICKYPICKY
					rectTop    = DTSCoord( rectTop    + dv );
					rectLeft   = DTSCoord( rectLeft   + dh );
					rectBottom = DTSCoord( rectBottom - dv );
					rectRight  = DTSCoord( rectRight  - dh );
#else
					rectTop    += dv;
				   	rectLeft   += dh;
				   	rectBottom -= dv;
				   	rectRight  -= dh;
#endif
					}
	
	// move to a new top and left preserving the size
	void		Move( DTSCoord left, DTSCoord top )
					{
#if PICKYPICKY
					rectBottom = DTSCoord( rectBottom + top  - rectTop );
					rectRight  = DTSCoord( rectRight  + left - rectLeft );
#else
					rectBottom += top - rectTop;
					rectRight += left - rectLeft;
#endif
					rectTop  = top;
				   	rectLeft = left;
					}
	
	// test if empty
	bool		IsEmpty() const
					{ return ( rectTop >= rectBottom ) || ( rectLeft >= rectRight ); }
	
	// expand to include the point
	void		Expand( const DTSPoint& pt )
					{ Expand( pt.ptH, pt.ptV ); }
	void		Expand( DTSCoord h, DTSCoord v );
	
	// expand to include another rectangle
	void		Union( const DTSRect * other );
	
	// shrink to intersection with another rectangle
	void		Intersect( const DTSRect * other );
	
	// test if equal
	bool		IsEqualTo( const DTSRect& otherRect ) const
					{ return rectTop == otherRect.rectTop
						&& rectLeft == otherRect.rectLeft
						&& rectBottom == otherRect.rectBottom
						&& rectRight == otherRect.rectRight; }
	
	// Is this rectangle contained in the parameter rectangle?
	bool		InRect( const DTSRect * inRect ) const
					{
					return 	rectTop    >= inRect->rectTop	&& rectTop    <= inRect->rectBottom
						&&	rectBottom >= inRect->rectTop	&& rectBottom <= inRect->rectBottom
						&&	rectLeft   >= inRect->rectLeft	&& rectLeft   <= inRect->rectRight
						&&	rectRight  >= inRect->rectLeft	&& rectRight  <= inRect->rectRight;
					}
};

#pragma pack( pop )


/*
**	DTSPoint::InRect()
**	is this point contained within the rect?
*/
inline bool
DTSPoint::InRect( const DTSRect * box ) const
{
	return
	  ( ptV >= box->rectTop
	&&  ptV <  box->rectBottom
	&&  ptH >= box->rectLeft
	&&  ptH <  box->rectRight );
};


/*
**	Derive DTSOval from DTSRect and add no additional methods
*/
class DTSOval : public DTSRect
{
};


/*
**	Derive DTSRound from DTSRect.
*/
class DTSRound : public DTSRect
{
public:
	DTSCoord		roundHeight;
	DTSCoord		roundWidth;
	
	// set the roundedness
	void		SetRound( DTSCoord width, DTSCoord height )
					{ roundHeight = height; roundWidth = width; }
};


/*
**	Derive DTSArc class from DTSRect.
*/
class DTSArc : public DTSRect
{
public:
	// stop and start angles mod 360
	DTSCoord		arcStart;
	DTSCoord		arcStop;
	
	// set the angles
	void		SetAngles( DTSCoord start, DTSCoord stop )
					{ arcStart = start; arcStop = stop; }
};


/*
**	DTSLine class
*/
class DTSLine
{
public:
	DTSPoint		linePt1;
	DTSPoint		linePt2;
	
	// set the endpoints of the line
	void		Set( const DTSPoint& pt1, const DTSPoint& pt2 )
					{ linePt1 = pt1; linePt2 = pt2; }
	void		Set( DTSCoord h1, DTSCoord v1, DTSCoord h2, DTSCoord v2 )
					{ linePt1.ptV = v1; linePt1.ptH = h1; linePt2.ptV = v2; linePt2.ptH = h2; }
	
	// offset the line
	void		Offset( DTSCoord dh, DTSCoord dv )
					{ linePt1.Offset( dh, dv ); linePt2.Offset( dh, dv ); }
};


#if 0	// Polygons? No! Polys gone!
/*
**	DTSPolygon class
*/
class DTSPolygonPriv;
class DTSPolygon
{
public:
	DTSImplement<DTSPolygonPriv> priv;
	
	// add a point to the polygon
	void		Add( const DTSPoint& pt ) { Add( pt.ptH, pt.ptV ); }
	void		Add( DTSCoord h, DTSCoord v );
	
	// offset
	void		Offset( DTSCoord dh, DTSCoord dv );
	
	// reset to empty
	void		Reset();
	
	// accessors
	void		GetBounds( DTSRect * oBounds ) const;
};
#endif  // 0


#ifndef AVOID_QUICKDRAW
// Regions are QD-only (MacOS 10.8 and higher can use CGPaths and HIShapes to
// get _some_ of the same functionality, but Unix builds don't need any of this)
/*
**	DTSRegion class
*/
class DTSRegionPriv;
class DTSRegion
{
public:
	DTSImplement<DTSRegionPriv> priv;
	
	// set the region to a specific shape
	void		SetEmpty();
	void		Set( const DTSRect * rect );
	void		Set( const DTSOval * oval );
	void		Set( const DTSRound * round );
//	void		Set( const DTSPolygon * poly );
	void		Set( const DTSRegion * rgn );
	
	// offset and move
	void		Offset( DTSCoord dh, DTSCoord dv );
	void		Move( DTSCoord horz, DTSCoord vert );
	
	// add another region
	void		Union( const DTSRegion * other );
	
	// intersect with another region 
	void		Intersect( const DTSRegion * other );
	
	// subtract another region
	void		Subtract( const DTSRegion * other );
	
	// xor with another region
	void		Xor( const DTSRegion * other );
	
	// accessors
	void		GetBounds( DTSRect * oBounds ) const;
};
#endif  // ! AVOID_QUICKDRAW


#ifndef AVOID_QUICKDRAW
// QD Pictures are QD-only (naturally enough)
/*
**	DTSPicture class
*/
class DTSPicturePriv;
class DTSPicture
{
public:
	DTSImplement<DTSPicturePriv> priv;
	
	// move to/from scrap
	DTSError		GetFromScrap();
	DTSError		CopyToScrap() const;
	
	// load the picture from a resource
	DTSError		Load( int id );
	
	// create from a view
	void			Set( DTSView * view, const DTSRect * box );
	
	// reset to nothing
	void			Reset();
	
	// accessors
	void			GetBounds( DTSRect * oBounds ) const;
};
#endif  // AVOID_QUICKDRAW


/*
**	DTSImage class
*/
class DTSImagePriv;
class DTSImage
{
public:
	DTSImplementNoCopy<DTSImagePriv> priv;
	
	// initialize (or re-initialize)
	// will save the bits pointer
	DTSError		Init( void * bits, DTSCoord width, DTSCoord height, int depth );
	
	// allocate memory for pixels
	DTSError		AllocateBits();
	
	// dispose of memory for pixels
	void			DisposeBits();
	
#ifndef AVOID_QUICKDRAW
	// load and initialize from a PICT resource
	DTSError		Load( int depth, int rsrcID );
	DTSError		GetFromScrap( int depth );
#endif  // ! AVOID_QUICKDRAW
	
	// accessors
	void			SetBits( void * bits );
	void *			GetBits() const;
	int				GetDepth() const;
	void			GetBounds( DTSRect * oBounds ) const;
	int				GetRowBytes() const;
};


/*
**	DTSFocus class
*/
class DTSFocus
{
public:
	// constructor/destructor
	virtual				~DTSFocus() {}
	
	// handle key strokes
	// KeyStroke() returns true if it handled the event
	// it returns false if the caller should do something with the event
	virtual bool		KeyStroke( int /* ch */, uint /* modifiers */ )
							{ return false; }
	
	// handle mouse clicks
	// Click() returns true if it handled the event
	// it returns false if the caller should do something with the event
	virtual bool		Click( const DTSPoint& /* loc */, ulong /* time */, uint /* modifiers */ )
							{ return false; }
	
	// this may get called periodically in certain case
	virtual void		Idle() {}
};


// these are modifier bits, as in keydown or click events;
// not keycodes (as in IsKeyDown())
enum
{
	kKeyModShift		= 0x0001,
	kKeyModControl		= 0x0002,
	kKeyModMenu			= 0x0004,
	kKeyModOption		= 0x0008,
	kKeyModRepeat		= 0x0010,
	kKeyModNumpad		= 0x0020,
	kKeyModCapsLock		= 0x0040
};


/*
**	DTSView class
**	base class for objects that can somehow be "drawn".
*/
class DTSViewPriv;
class DTSView : public DTSFocus
{
public:
	DTSImplementNoCopy<DTSViewPriv> priv;
	
	// attach a view to another
	void		Attach( DTSView * parent );
	
	// detach from parent
	void		Detach();
	
	// reset the drawing environment to default values
	// ie no clipping, black pattern, pen mode copy, etc.
	void		Reset();
	
	// set a clipping region for all drawing operations
#ifndef AVOID_QUICKDRAW
	void		GetMask( DTSRegion * outMask ) const;
	void		SetMask( const DTSRegion * mask );
	void		SetMask( const DTSRect * rect );
#endif
	
	// set the size of the pen for line and frame drawing operations
	// set the pen drawing mode to kPenModeXXX
	// set the pattern for shape drawing
	// set the two colors for patterns
	void		SetPenSize( DTSCoord size );
	void		SetPenMode( int mode );
#ifndef AVOID_QUICKDRAW
	void		SetPattern( const DTSPattern * pattern );
#endif
	void		SetForeColor( const DTSColor * color );
//	void		SetForeColor( int index );
	void		SetBackColor( const DTSColor * color );
//	void		SetBackColor( int index );
	void		SetHiliteColor( const DTSColor * color );
//	void		SetHiliteColor( int index );
	
	// set temporary user-hilite-color mode for the next Invert()
	void		SetHiliteMode();
	
	// set the blend parameter from 0x0000 (no source) to 0xFFFF (no destination)
	// applies to Frame verbs, Paint verbs, Draw( DTSLine ) when using the kPenModeBlend
	// and to Draw( DTSImage, kImageBlend );
	void		SetBlend( int value );
	
	// set the font by name
	// set font size
	// set font style
	// get the width of a text string
	void		SetFont( const char * fontName );
	void		SetFontSize( DTSCoord size );
	void		SetFontStyle( int style );
	DTSCoord	GetTextWidth( const char * text ) const;
	
	// draw commands
	void		Frame( const DTSRect * rect );
	void		Frame( const DTSOval * oval );
	void		Frame( const DTSRound * round );
	void		Frame( const DTSArc * arc );
#ifndef AVOID_QUICKDRAW
//	void		Frame( const DTSPolygon * poly );
	void		Frame( const DTSRegion * rgn );
#endif
	void		Paint( const DTSRect * rect );
	void		Paint( const DTSOval * oval );
	void		Paint( const DTSRound * round );
	void		Paint( const DTSArc * arc );
#ifndef AVOID_QUICKDRAW
//	void		Paint( const DTSPolygon * poly );
	void		Paint( const DTSRegion * rgn );
#endif
	void		Erase( const DTSRect * rect );
	void		Erase( const DTSOval * oval );
	void		Erase( const DTSRound * round );
	void		Erase( const DTSArc * arc );
#ifndef AVOID_QUICKDRAW
//	void		Erase( const DTSPolygon * poly );
	void		Erase( const DTSRegion * rgn );
#endif
	void		Invert( const DTSRect * rect );
	void		Invert( const DTSOval * oval );
	void		Invert( const DTSRound * round );
	void		Invert( const DTSArc * arc );
#ifndef AVOID_QUICKDRAW
//	void		Invert( const DTSPolygon * poly );
	void		Invert( const DTSRegion * rgn );
#endif
	void		Draw( const DTSLine * line );
	// draw the image into this view
	void		Draw( const DTSImage * img, int mode, const DTSRect * src, const DTSRect * dst );
	// copy (a part of) a view into (part of) another
	void		Draw( const DTSView * view, int mode, const DTSRect * src, const DTSRect * dst );
	
	// draw a bit of text
	void		Draw( const char * text, DTSCoord h, DTSCoord v, int just );
	
#ifndef AVOID_QUICKDRAW
	// draw a picture into the view
	void		Draw( const DTSPicture * picture, const DTSRect * dst );
#endif
	
	// show this view, draw it and all of its visible children recursively
	// hide this view, put do not erase it or any of its children
	virtual void	Show();
	virtual void	Hide();
	
	// draw this view and all of its children
	// Draw sets up the drawing environment in preparation for the drawing commands.
	// DoDraw is usually overridden and calls the drawing commands.
	// DoDraw does not set up the drawing environment.
	// calling DoDraw directly is an invitation to crash and burn
	// DTSView::DoDraw calls DoDraw for all of its visible children
	void			Draw();
	virtual void	DoDraw();
	
	// move to new top left
	// set size
	virtual void	Move( DTSCoord h, DTSCoord v );
	virtual void	Size( DTSCoord width, DTSCoord height );
	
	// semi-private
	// overloaded DTSFocus routines
	virtual bool	Click( const DTSPoint& loc, ulong when, uint modifiers );
	
	// accessors
	DTSView *	GetParent() const;			// parent of this view					default nil
	DTSView *	GetChild() const;			// first child of this view				default nil
	DTSView *	GetSibling() const;			// next child of parent view			default nil	
	void		GetBounds( DTSRect * oBounds ) const;	// bounding rectangle		default 0,0,0,0
	bool		GetVisible() const;			// true if view is visible				default false
									// visible children of invisible parents will NOT be drawn
#ifndef AVOID_QUICKDRAW
	void		GetContent( DTSRegion * oRgn ) const;	// content region of the view
#endif
};

enum
{
	// pen modes
	kPenModeCopy			= 0,
	kPenModeTransparent,
	kPenModeErase,
	kPenModeInvert,
	kPenModeBlend,
	
	// pattern modes
	kImageCopy				= 0,
	kImageCopyFast,
	kImageCopyQuality,
	kImageTransparent,
	kImageErase,
	kImageBlend,
	kImageTransparentQuality
};

enum
{
	// text styles
	kStyleNormal			= 0x0000,
	kStyleBold				= 0x0001,
	kStyleItalic			= 0x0002,
	kStyleUnderline			= 0x0004,
	kStyleOutline			= 0x0008,
	kStyleShadow			= 0x0010,
	kStyleCondense			= 0x0020,
	kStyleExtend			= 0x0040,
	
	// text justifications (porting note: these are NOT 1-for-1 identical
	// to Mac teFlushLeft, Center, etc.)
	kJustLeft				= 0,
	kJustRight,
	kJustCenter
};


#ifndef AVOID_QUICKDRAW
/*
**	DTSOffView class derived from DTSView
*/
class DTSOffView : public DTSView
{
private:
	// declared but not defined, to prevent inadvertent copying.
					DTSOffView( const DTSOffView & );
	DTSOffView&		operator=( const DTSOffView & );
	
public:
	DTSImage *		offImage;
	
	// constructor/destructor
				DTSOffView();
	virtual		~DTSOffView();
	
	// initialize to a specific depth
	void		Init( int depth );
	
	// set the image
	void		SetImage( DTSImage * image );
	
	// pixels commands
	void		Set1Pixel( DTSCoord h, DTSCoord v, const DTSColor * color );
	void		Set1Pixel( DTSCoord h, DTSCoord v, int index );
	void		Get1Pixel( DTSCoord h, DTSCoord v, DTSColor * oColor ) const;
	int			Get1Pixel( DTSCoord h, DTSCoord v ) const;
};
#endif  // ! AVOID_QUICKDRAW

#endif	// View_dts_h

