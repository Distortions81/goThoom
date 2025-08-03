/*
**	View_mac.h		dtslib2
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

#ifndef View_mac_h
#define View_mac_h

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "View_dts.h"


#ifndef AVOID_QUICKDRAW
/*
**	DTSPatternPriv
*/
class DTSPatternPriv
{
public:
	PixPatHandle	patHdl;
	Pattern			pattern;
	
	// constructor/destructor
						DTSPatternPriv();
						~DTSPatternPriv();
	
	// interface
	void				Set( uint bw );
	void				Reset();
};


# if 0	// no more polygons
/*
**	DTSPolygonPriv
*/
class DTSPolygonPriv
{
public:
	PolyHandle		polyHdl;
	DTSRect			polyBounds;
	
	// constructor/destructor
						DTSPolygonPriv();
						DTSPolygonPriv( const DTSPolygonPriv & rhs );
						~DTSPolygonPriv();
	DTSPolygonPriv&		operator=( const DTSPolygonPriv & rhs );
};
# endif  // 0


/*
**	DTSRegionPriv
*/
class DTSRegionPriv
{
public:
	RgnHandle		rgnHdl;
	
	// constructor/destructor
						DTSRegionPriv();
						DTSRegionPriv( const DTSRegionPriv & rhs );
						~DTSRegionPriv();
	DTSRegionPriv&		operator=( const DTSRegionPriv & rhs );
};


/*
**	DTSPicturePriv
*/
class DTSPicturePriv
{
public:
	PicHandle		picHdl;
	DTSBoolean		picResource;
	
	// constructor/destructor
						DTSPicturePriv();
						DTSPicturePriv( const DTSPicturePriv & rhs );
						~DTSPicturePriv();
	DTSPicturePriv&		operator=( const DTSPicturePriv & rhs );
	
	// interface
	void				Reset();
};
#endif  // ! AVOID_QUICKDRAW


/*
**	DTSImagePriv
*/
class DTSImagePriv
{
public:
	enum AllocKind
		{
		ipAllocUnknown = 0,			// unknown bits source
		ipAllocNew,					// bits were new[]'d, must delete[]
		ipAllocPtr,					// from NewPtr()
		ipAllocNone					// no bits at all, don't dealloc
		};
	
	PixMap			imageMap;
	AllocKind		imageAllocated;
	int				imageRowBytes;
	
	// constructor/destructor
						DTSImagePriv();
						~DTSImagePriv();
	
	// interface
	DTSError			AllocateBits();
	void				DisposeBits();
};


/*
**	DTSViewPriv
*/
class DTSViewPriv
{
public:
	GrafPtr			viewPort;
	GDHandle		viewDevice;
	DTSView *		viewParent;
	DTSView *		viewChild;
	DTSView *		viewSibling;
	DTSRect			viewBounds;
	bool			viewVisible;
#ifndef AVOID_QUICKDRAW
	DTSRegion		viewContent;
#endif
	
	// constructor/destructor
						DTSViewPriv();
						~DTSViewPriv();
	
	// interface
	void				InitViewPort( GrafPtr port, GDHandle device );
	void				Detach();
};


/*
**	StDrawEnv
*/
class StDrawEnv
{
public:
	CGrafPtr		envPort;
	GDHandle		envDevice;
	
	// constructor/destructor
				StDrawEnv( GrafPtr   port, GDHandle device = nullptr );
	// in carbon, GrafPtr == CGrafPtr == OpaqueGrafPtr != WindowRef != DialogRef
				StDrawEnv( WindowRef port, GDHandle device = nullptr );
				StDrawEnv( DialogRef port, GDHandle device = nullptr );
	
	explicit	StDrawEnv( const DTSView * view );
				~StDrawEnv();
};


/*
**	Routines
*/
#ifndef AVOID_QUICKDRAW
CTabHandle	MyGetCTable( int depth );
void		SyncPaletteTable();
#endif  // ! AVOID_QUICKDRAW


/*
**	Conversions from DTS types to Macintosh types (not endian-safe)
*/
inline       RGBColor *   DTS2Mac(       DTSColor   * c ) { return ( RGBColor *) &c->rgbRed; }
inline const RGBColor *   DTS2Mac( const DTSColor   * c ) { return ( const RGBColor *) &c->rgbRed; }
inline       Point *      DTS2Mac(       DTSPoint   * p ) { return ( Point *) &p->ptV; }
inline const Point *      DTS2Mac( const DTSPoint   * p ) { return ( const Point *) &p->ptV; }
inline       Rect *       DTS2Mac(       DTSRect    * r ) { return ( Rect *) &r->rectTop; }
inline const Rect *       DTS2Mac( const DTSRect    * r ) { return ( const Rect *) &r->rectTop; }
#ifndef AVOID_QUICKDRAW
inline       PixPatHandle DTS2Mac( const DTSPattern * p )
				{ return ( p->priv.p ) ? p->priv.p->patHdl : nullptr; }
//inline     PolyHandle   DTS2Mac( const DTSPolygon * p )
//				{ return ( p->priv.p ) ? p->priv.p->polyHdl : nullptr; }
inline       RgnHandle    DTS2Mac( const DTSRegion  * r )
				{ return ( r->priv.p ) ? r->priv.p->rgnHdl : nullptr; }
inline       PicHandle    DTS2Mac( const DTSPicture * p )
				{ return ( p->priv.p ) ? p->priv.p->picHdl : nullptr; }
inline       BitMap *     DTS2Mac(       DTSImage   * i )
				{ return ( i->priv.p ) ? (BitMap *) &i->priv.p->imageMap : nullptr; }
inline const BitMap *     DTS2Mac( const DTSImage   * i )
				{ return ( i->priv.p ) ? (const BitMap *) &i->priv.p->imageMap : nullptr; }
inline 		 GrafPtr	  DTS2Mac( const DTSView    * v )
				{ return ( v->priv.p ) ? v->priv.p->viewPort : nullptr; }
#endif  // ! AVOID_QUICKDRAW

#endif	// View_mac_h

