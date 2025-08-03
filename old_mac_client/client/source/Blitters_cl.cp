/*
**	Blitters_cl.cp		ClanLord
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

#include "ClanLord.h"


/*
**	Entry Routines
*/
/*
void	InitBlitters();
void	BlitTransparent( DTSOffView *, DTSImage *, const DTSRect *, const DTSRect * );
void	QualityBlitBlend25( DTSOffView *, DTSImage *, const DTSRect *, const DTSRect * );
void	QualityBlitBlend25Transparent( DTSOffView *, DTSImage *, const DTSRect *, const DTSRect *);
void	QualityBlitBlend50( DTSOffView *, DTSImage *, const DTSRect *, const DTSRect * );
void	QualityBlitBlend50Transparent( DTSOffView *, DTSImage *, const DTSRect *, const DTSRect *);
void	QualityBlitBlend75( DTSOffView *, DTSImage *, const DTSRect *, const DTSRect * );
void	QualityBlitBlend75Transparent( DTSOffView *, DTSImage *, const DTSRect *, const DTSRect *);
void	FastBlitBlend25( DTSOffView *, DTSImage *, const DTSRect *, const DTSRect * );
void	FastBlitBlend25Transparent( DTSOffView *, DTSImage *, const DTSRect *, const DTSRect * );
void	FastBlitBlend50( DTSOffView *, DTSImage *, const DTSRect *, const DTSRect * );
void	FastBlitBlend50Transparent( DTSOffView *, DTSImage *, const DTSRect *, const DTSRect * );
void	FastBlitBlend75( DTSOffView *, DTSImage *, const DTSRect *, const DTSRect * );
void	FastBlitBlend75Transparent( DTSOffView *, DTSImage *, const DTSRect *, const DTSRect * );
*/

/*
**	Definitions
*/
#if 0
static uchar *	InitBlendTable( DTSOffView * view, int destweight );
static void		InitBlendTables( DTSOffView * view );
#endif

const int kCompSize		= 6 * 4;
//const int kTableSize	= kCompSize * kCompSize * kCompSize / 4;
const int kBlendDither	= 2 * kCompSize * kCompSize + 2 * kCompSize + 2;


/*
**	Internal Routines
*/


/*
**	Internal Variables
*/
//static uchar *	gBlend25Table;
//static uchar *	gBlend50Table;
static ushort *	gBlendRGBTable;
static uchar *	gBlendInvTable;


/*
**	InitBlitters()
**
**
*/
void
InitBlitters()
{
#if 0
	GetAppData( 'BTbl', 1, &gBlend25Table, nullptr );
	GetAppData( 'BTbl', 2, &gBlend50Table, nullptr );
#endif
	
	GetAppData( 'BTbl', 3, (void**) &gBlendRGBTable, nullptr );
	GetAppData( 'BTbl', 4, (void**) &gBlendInvTable, nullptr );
	
#if DTS_LITTLE_ENDIAN
	// byte-swap the RGB table
	int nItems = 256;
	uint16_t * p = gBlendRGBTable;
	while ( nItems-- > 0 )
		{
		*p = BigToNativeEndian( *p );
		++p;
		}
#endif  // DTS_LITTLE_ENDIAN
}


/*
**	BlitClipApparatus()
**
**	initialize the variables for a blit routine
**	return false if there is nothing to blit
*/
static inline bool
BlitClipApparatus(
	const DTSOffView * view, const DTSImage * srcimage,
	const DTSRect * srcrect, const DTSRect * dstrect,
	int & height, int &width, int & srcrowbytes, int & dstrowbytes,
	const uchar * & srcbits, uchar * & dstbits )
{
	
	// pin the destination rectangle to be within the destination image
	// shrink the source rectangle accordingly
	const DTSImage * dstimage = view->offImage;
	DTSRegion cliprgn;
	view->GetMask( &cliprgn );
	int dsttop  = dstrect->rectTop;
	int dstleft = dstrect->rectLeft;
	int srctop  = srcrect->rectTop;
	int srcleft = srcrect->rectLeft;
	height  = dstrect->rectBottom - dsttop;
	width   = dstrect->rectRight  - dstleft;
	DTSRect clipBounds;
	cliprgn.GetBounds( &clipBounds );
	int diff = clipBounds.rectTop - dsttop;
	if ( diff > 0 )
		{
		height  -= diff;
		dsttop  += diff;
		srctop  += diff;
		}
	diff = clipBounds.rectLeft - dstleft;
	if ( diff > 0 )
		{
		width   -= diff;
		dstleft += diff;
		srcleft += diff;
		}
	diff = dstrect->rectBottom - clipBounds.rectBottom;
	if ( diff > 0 )
		{
		height  -= diff;
		}
	diff = dstrect->rectRight - clipBounds.rectRight;
	if ( diff > 0 )
		{
		width   -= diff;
		}
	
	// bail if we have nothing to do
	if ( height <= 0
	||   width  <= 0 )
		{
		return false;
		}
	
	// get a pointer to the first pixel in the source
	srcrowbytes = srcimage->GetRowBytes();
	DTSRect srcimageBounds;
	srcimage->GetBounds( &srcimageBounds );
	srcbits  = ((uchar *) srcimage->GetBits())
		+ srcrowbytes * ( srctop - srcimageBounds.rectTop )
		+ ( srcleft - srcimageBounds.rectLeft );
	
	// get a pointer to the first pixel in the destination
	dstrowbytes = dstimage->GetRowBytes();
	DTSRect dstimageBounds;
	dstimage->GetBounds( &dstimageBounds );
	dstbits = ((uchar *) dstimage->GetBits())
		+ dstrowbytes * ( dsttop - dstimageBounds.rectTop )
		+ ( dstleft - dstimageBounds.rectLeft );
	
	// change rowbytes to offsets
	srcrowbytes -= width;
	dstrowbytes -= width;
	return true;
}


#if 0
/*
**	BlitTransparentRunB()
**
**	blit a run of bytes
*/
static inline void
BlitTransparentRunB(
	const uchar * &	srcbits,
	uchar * &		dstbits,
	int				count )
{
	for ( int p = 0; p < count; ++p )
	{
		uchar src = *srcbits;
		if ( src )
			*dstbits = src;
		++srcbits;
		++dstbits;
	}
}
#endif  // 0


/*
**	BlitTransparent()
**
**	draw offscreen transparent faster than the slow versions of quickdraw
**	assume source and destination are both default 8 bit cluts
**	assume no scaling
**	assume no stretching
**	assume source rect is completely within the source image bounds
**	assume we might need to clip the source rect so the destination fits in the 
**		destination image bounds
**	assume a rectangular clipping region smaller than the destination image bounds
*/
void
BlitTransparent( const DTSOffView * view, const DTSImage * srcimage,
	const DTSRect * srcrect, const DTSRect * dstrect )
{
// timing tests
#if 1
	// return immediately to establish a baseline draw time just for the overhead.
	//return;
	
	// draw 100 consecutive times to expand the sensitivity of the draw time measurement.
	//for ( int testcnt = 100;  testcnt > 0;  --testcnt )
	{
#endif

	int height, width, srcrowbytes, dstrowbytes;
	const uchar * srcbits;
	uchar * dstbits;
	
	if ( ! BlitClipApparatus( view, srcimage, srcrect, dstrect,
			height, width, srcrowbytes, dstrowbytes, srcbits, dstbits ) )
		return;
	
#if 0
	{
		// for each row
		for ( ;  height > 0;  --height )
			{
			// for each pixel
			BlitTransparentRunB( srcbits, dstbits, width );
			
			// offset to next row
			srcbits += srcrowbytes;
			dstbits += dstrowbytes;
			}
	}
#else
	// byte based transparent blitter
	// manually unroll the loop
	// and manually schedule instructions.
	// yeah, yeah i know it looks pretty scary to nest a for loop inside
	// a switch statement but it works, it's compact, and it's fast
	// and i think it's perty.
	// 61 (38) ms
	{
		// number of times to loop through the block
		// including the first partial block.
		int nloops  = ( width - 1 ) >> 3;
		
		// where to start in the first partial block.
		int start   = 9 - ( width - ( nloops << 3 ) );
		
		// modify pointers and offsets as if the first partial
		// block was a full block.
		// adjust for the trailing pixel.
		srcbits     -= start;
		dstbits     -= start;
		srcrowbytes -= start - 1;
		dstrowbytes -= start - 1;
		
		// for each row
		for ( ;  height > 0;  --height )
			{
			// initialize the pixel holder
			uchar pixel0;
			uchar pixel1;
			pixel0 = pixel1 = srcbits[start];
			
			// switch into the right place for first partial block
			int count = nloops;
			switch ( start )
				{
				// unroll the loop 8 times.
				// unrolling more gives marginal improvement.
				// compilers optimize for loop counters like this easily.
				for ( ;  count >= 0;  --count )
					{
					// interleave the load of the next pixel with the test 
					// and store of the previous pixel.
					// interleaving more than two pixels like this gives 
					// no improvement.
				        pixel1 = srcbits[ 1]; if ( pixel0 ) dstbits[ 0] = pixel0;
				case 1: pixel0 = srcbits[ 2]; if ( pixel1 ) dstbits[ 1] = pixel1;
				case 2: pixel1 = srcbits[ 3]; if ( pixel0 ) dstbits[ 2] = pixel0;
				case 3: pixel0 = srcbits[ 4]; if ( pixel1 ) dstbits[ 3] = pixel1;
				case 4: pixel1 = srcbits[ 5]; if ( pixel0 ) dstbits[ 4] = pixel0;
				case 5: pixel0 = srcbits[ 6]; if ( pixel1 ) dstbits[ 5] = pixel1;
				case 6: pixel1 = srcbits[ 7]; if ( pixel0 ) dstbits[ 6] = pixel0;
				case 7: pixel0 = srcbits[ 8]; if ( pixel1 ) dstbits[ 7] = pixel1;
					
					// bump the pointers
				default:
					srcbits += 8;
					dstbits += 8;
					}
				}
			
			// test and store the trailing pixel.
			if ( pixel0 ) dstbits[ 0] = pixel0;
			
			// offset to next row
			srcbits += srcrowbytes;
			dstbits += dstrowbytes;
			}
	}
#endif

#if 1
	} // timing tests
#endif
}


/*
**	QualityBlitBlend25()
**
**	draw offscreen transparent faster than the slow versions of quickdraw
**	assume source and destination are both default 8 bit cluts
**	assume no scaling
**	assume no stretching
**	assume source rect is completely within the source image bounds
**	assume we might need to clip the source rect so the destination fits in the 
**		destination image bounds
**	assume a rectangular clipping region smaller than the destination image bounds
*/
void
QualityBlitBlend25( const DTSOffView * view, const DTSImage * srcimage,
	const DTSRect * srcrect, const DTSRect * dstrect )
{
	int height, width, srcrowbytes, dstrowbytes;
	const uchar * srcbits;
	uchar * dstbits;
	
	if ( ! BlitClipApparatus( view, srcimage, srcrect, dstrect, height, width,
			srcrowbytes, dstrowbytes, srcbits, dstbits ) )
		return;
	
	// for each row
	const ushort * rgbtable = gBlendRGBTable;
	const uchar * invtable  = gBlendInvTable;
	int dither = kBlendDither;
	for ( ;  height > 0;  --height )
		{
		// for each pixel
		for ( int diff = width;  diff > 0;  --diff )
			{
			int wanted = rgbtable[ *srcbits++ ];
			wanted    += ( wanted << 1 ) +  rgbtable[ *dstbits ] + dither;
			int pixel  = invtable[ wanted >> 2 ];
			*dstbits++ = pixel;
			dither     = wanted - ( rgbtable[ pixel ] << 2 );
			}
		
		// offset to next row
		srcbits += srcrowbytes;
		dstbits += dstrowbytes;
		}
}


/*
**	QualityBlitBlend25Transparent()
**
**	draw offscreen transparent faster than the slow versions of quickdraw
**	assume source and destination are both default 8 bit cluts
**	assume no scaling
**	assume no stretching
**	assume source rect is completely within the source image bounds
**	assume we might need to clip the source rect so the destination fits in the 
**		destination image bounds
**	assume a rectangular clipping region smaller than the destination image bounds
*/
void
QualityBlitBlend25Transparent( const DTSOffView * view, const DTSImage * srcimage,
	const DTSRect * srcrect, const DTSRect * dstrect )
{
	int height, width, srcrowbytes, dstrowbytes;
	const uchar * srcbits;
	uchar * dstbits;
	
	if ( ! BlitClipApparatus( view, srcimage, srcrect, dstrect, height, width,
			srcrowbytes, dstrowbytes, srcbits, dstbits ) )
		return;
	
	// for each row
	const ushort * rgbtable = gBlendRGBTable;
	const uchar * invtable  = gBlendInvTable;
	int dither = kBlendDither;
	for ( ;  height > 0;  --height )
		{
		// for each pixel
		for ( int diff = width;  diff > 0;  --diff )
			{
			int pixel = *srcbits++;
			if ( pixel )
				{
				int wanted = rgbtable[ pixel ];
				wanted  += ( wanted << 1 ) +  rgbtable[ *dstbits ] + dither;
				pixel    = invtable[ wanted >> 2 ];
				*dstbits = pixel;
				dither   = wanted - ( rgbtable[ pixel ] << 2 );
				}
			++dstbits;
			}
		
		// offset to next row
		srcbits += srcrowbytes;
		dstbits += dstrowbytes;
		}
}


/*
**	QualityBlitBlend50()
**
**	draw offscreen transparent faster than the slow versions of quickdraw
**	assume source and destination are both default 8 bit cluts
**	assume no scaling
**	assume no stretching
**	assume source rect is completely within the source image bounds
**	assume we might need to clip the source rect so the destination fits in the 
**		destination image bounds
**	assume a rectangular clipping region smaller than the destination image bounds
*/
void
QualityBlitBlend50( const DTSOffView * view, const DTSImage * srcimage,
	const DTSRect * srcrect, const DTSRect * dstrect )
{
	int height, width, srcrowbytes, dstrowbytes;
	const uchar * srcbits;
	uchar * dstbits;
	
	if ( ! BlitClipApparatus( view, srcimage, srcrect, dstrect, height, width,
			srcrowbytes, dstrowbytes, srcbits, dstbits ) )
		return;
	
	// for each row
	const ushort * rgbtable = gBlendRGBTable;
	const uchar * invtable  = gBlendInvTable;
	int dither = kBlendDither;
	for ( ;  height > 0;  --height )
		{
		// for each pixel
		for ( int diff = width;  diff > 0;  --diff )
			{
			int wanted = ( ( rgbtable[ *srcbits++ ] + rgbtable[ *dstbits ] ) << 1 ) + dither;
			int pixel  = invtable[ wanted >> 2 ];
			*dstbits++ = pixel;
			dither     = wanted - ( rgbtable[ pixel ] << 2 );
			}
		
		// offset to next row
		srcbits += srcrowbytes;
		dstbits += dstrowbytes;
		}
}


/*
**	QualityBlitBlend50Transparent()
**
**	draw offscreen transparent faster than the slow versions of quickdraw
**	assume source and destination are both default 8 bit cluts
**	assume no scaling
**	assume no stretching
**	assume source rect is completely within the source image bounds
**	assume we might need to clip the source rect so the destination fits in the 
**		destination image bounds
**	assume a rectangular clipping region smaller than the destination image bounds
*/
void
QualityBlitBlend50Transparent( const DTSOffView * view, const DTSImage * srcimage,
	const DTSRect * srcrect, const DTSRect * dstrect )
{
	int height, width, srcrowbytes, dstrowbytes;
	const uchar * srcbits;
	uchar * dstbits;
	
	if ( ! BlitClipApparatus( view, srcimage, srcrect, dstrect, height, width,
			srcrowbytes, dstrowbytes, srcbits, dstbits ) )
		return;
	
	// for each row
	const ushort * rgbtable = gBlendRGBTable;
	const uchar * invtable  = gBlendInvTable;
	int dither = kBlendDither;
	for ( ;  height > 0;  --height )
		{
		// for each pixel
		for ( int diff = width;  diff > 0;  --diff )
			{
			int pixel = *srcbits++;
			if ( pixel )
				{
				int wanted = ( ( rgbtable[ pixel ] + rgbtable[ *dstbits ] ) << 1 ) + dither;
				pixel    = invtable[ wanted >> 2 ];
				*dstbits = pixel;
				dither   = wanted - ( rgbtable[ pixel ] << 2 );
				}
			++dstbits;
			}
		
		// offset to next row
		srcbits += srcrowbytes;
		dstbits += dstrowbytes;
		}
}


/*
**	QualityBlitBlend75()
**
**	draw offscreen transparent faster than the slow versions of quickdraw
**	assume source and destination are both default 8 bit cluts
**	assume no scaling
**	assume no stretching
**	assume source rect is completely within the source image bounds
**	assume we might need to clip the source rect so the destination fits in the 
**		destination image bounds
**	assume a rectangular clipping region smaller than the destination image bounds
*/
void
QualityBlitBlend75( const DTSOffView * view, const DTSImage * srcimage,
	const DTSRect * srcrect, const DTSRect * dstrect )
{
	int height, width, srcrowbytes, dstrowbytes;
	const uchar * srcbits;
	uchar * dstbits;
	
	if ( ! BlitClipApparatus( view, srcimage, srcrect, dstrect, height, width,
			srcrowbytes, dstrowbytes, srcbits, dstbits ) )
		return;
	
	// for each row
	const ushort * rgbtable = gBlendRGBTable;
	const uchar * invtable  = gBlendInvTable;
	int dither = kBlendDither;
	for ( ;  height > 0;  --height )
		{
		// for each pixel
		for ( int diff = width;  diff > 0;  --diff )
			{
			int wanted = rgbtable[ *dstbits ];
			wanted    += ( wanted << 1 ) +  rgbtable[ *srcbits++ ] + dither;
			int pixel  = invtable[ wanted >> 2 ];
			*dstbits++ = pixel;
			dither     = wanted - ( rgbtable[ pixel ] << 2 );
			}
		
		// offset to next row
		srcbits += srcrowbytes;
		dstbits += dstrowbytes;
		}
}


/*
**	QualityBlitBlend75Transparent()
**
**	draw offscreen transparent faster than the slow versions of quickdraw
**	assume source and destination are both default 8 bit cluts
**	assume no scaling
**	assume no stretching
**	assume source rect is completely within the source image bounds
**	assume we might need to clip the source rect so the destination fits in the 
**		destination image bounds
**	assume a rectangular clipping region smaller than the destination image bounds
*/
void
QualityBlitBlend75Transparent( const DTSOffView * view, const DTSImage * srcimage,
	const DTSRect * srcrect, const DTSRect * dstrect )
{
	int height, width, srcrowbytes, dstrowbytes;
	const uchar * srcbits;
	uchar * dstbits;
	
	if ( ! BlitClipApparatus( view, srcimage, srcrect, dstrect, height, width,
			srcrowbytes, dstrowbytes, srcbits, dstbits ) )
		return;
	
	// for each row
	const ushort * rgbtable = gBlendRGBTable;
	const uchar * invtable  = gBlendInvTable;
	int dither = kBlendDither;
	for ( ;  height > 0;  --height )
		{
		// for each pixel
		for ( int diff = width;  diff > 0;  --diff )
			{
			int pixel = *srcbits++;
			if ( pixel )
				{
				int wanted = rgbtable[ *dstbits ];
				wanted  += ( wanted << 1 ) +  rgbtable[ pixel ] + dither;
				pixel    = invtable[ wanted >> 2 ];
				*dstbits = pixel;
				dither   = wanted - ( rgbtable[ pixel ] << 2 );
				}
			++dstbits;
			}
		
		// offset to next row
		srcbits += srcrowbytes;
		dstbits += dstrowbytes;
		}
}


#if 0
/*
**	InitBlendTable()
**
**	initialize a blend table
*/
static uchar *
InitBlendTable( DTSOffView * view, int dstwt )
{
	uchar * table = NEW_TAG("InitBlendTable") uchar[ 256*256 ];
	if ( table )
		{
		int savepixel = view->Get1Pixel( 0, 0 );
		int srcwt = 4 - dstwt;
		uchar * step = table;
		DTSColor colortable[256];
		const DTSColor * srccolor = colortable;
		for ( int srcpixel = 0;  srcpixel < 256;  ++srcpixel )
			{
			DTSColor * dstcolor = colortable;
			for ( int dstpixel = 0;  dstpixel < 256;  ++dstpixel )
				{
				if ( 0 == srcpixel )
					{
					view->Set1Pixel( 0, 0, dstpixel );
					view->Get1Pixel( 0, 0, dstcolor );
					}
				
				DTSColor color;
				color.rgbRed   = ( srccolor->rgbRed   * srcwt + dstcolor->rgbRed   * dstwt ) >> 2;
				color.rgbGreen = ( srccolor->rgbGreen * srcwt + dstcolor->rgbGreen * dstwt ) >> 2;
				color.rgbBlue  = ( srccolor->rgbBlue  * srcwt + dstcolor->rgbBlue  * dstwt ) >> 2;
				view->Set1Pixel( 0, 0, &color );
				*step++ = view->Get1Pixel( 0, 0 );
				
				++dstcolor;
				}
			++srccolor;
			}
		
		view->Set1Pixel( 0, 0, savepixel );
		}
	
	return table;
}


/*
**	InitBlendTables()
**
**	initialize the blend table
*/
static void
InitBlendTables( DTSOffView * view )
{
	static bool bDoneAlready;
	if ( bDoneAlready )
		return;
	
	uchar * invtable = NEW_TAG("InitBlendTables") uchar[ kTableSize ];
	if ( ! invtable )
		return;
	
	bDoneAlready = true;
	ushort rgbtable[ 256 ];
	int savepixel = view->Get1Pixel( 0, 0 );
	
	DTSColor color;
	ushort * rgbt = rgbtable;
	for ( int index = 0;  index < 256;  ++index )
		{
		view->Set1Pixel( 0, 0, index );
		view->Get1Pixel( 0, 0, &color );
		
		// 0x0000 <= color.rgb <= 0xFFFF
		// 0 <= ( color.rgb >> 12 ) <= 15
		// 0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15
		// 0 0 1 1 1 2 2 2 3 3  3  4  4  4  5  5
		int r = ( ( color.rgbRed   >> 12 ) + 1 ) / 3;
		int g = ( ( color.rgbGreen >> 12 ) + 1 ) / 3;
		int b = ( ( color.rgbBlue  >> 12 ) + 1 ) / 3;
		
		if ( index < 6*6*6-1
		||   index == 255 )
			{
			for ( int nnn = 0;  nnn < 4;  ++nnn )
				{
				for ( int mmm = 0;  mmm < 4;  ++mmm )
					{
					int offset;
					offset  = ( ( r << 2 ) + nnn ) * kCompSize*kCompSize/4;
					offset += ( ( g << 2 ) + mmm ) * kCompSize/4;
					offset += b;
					invtable[offset] = index;
					}
				}
			}
		
		int temp = r * (kCompSize * kCompSize) + g * kCompSize + b;
		*rgbt++ = temp;
		}
	
	PutAppData( 'BTbl', 3, rgbtable, 2*256 );
	PutAppData( 'BTbl', 4, invtable, kTableSize );
	
	view->Set1Pixel( 0, 0, savepixel );
	delete[] invtable;
}
#endif  // 0


/*
**	FastBlitBlend25()
**
**	draw offscreen transparent faster than the slow versions of quickdraw
**	assume source and destination are both default 8 bit cluts
**	assume no scaling
**	assume no stretching
**	assume source rect is completely within the source image bounds
**	assume we might need to clip the source rect so the destination fits in the 
**		destination image bounds
**	assume a rectangular clipping region smaller than the destination image bounds
*/
void
FastBlitBlend25( const DTSOffView * view, const DTSImage * srcimage,
	const DTSRect * srcrect, const DTSRect * dstrect )
{
	// pin the destination rectangle to be within the destination image
	// shrink the source rectangle accordingly
	const DTSImage * dstimage = view->offImage;
	DTSRegion cliprgn;
	view->GetMask( &cliprgn );
	int dsttop  = dstrect->rectTop;
	int dstleft = dstrect->rectLeft;
	int srctop  = srcrect->rectTop;
	int srcleft = srcrect->rectLeft;
	int height  = dstrect->rectBottom - dsttop;
	int width   = dstrect->rectRight  - dstleft;
	DTSRect clipBounds;
	cliprgn.GetBounds( &clipBounds );
	int diff = clipBounds.rectTop - dsttop;
	int rowinset = 3;
	if ( diff > 0 )
		{
		height  -= diff;
		dsttop  += diff;
		srctop  += diff;
		if ( diff & 1 )
			rowinset = 1;
		}
	diff = clipBounds.rectLeft - dstleft;
	if ( diff > 0 )
		{
		width   -= diff;
		dstleft += diff;
		srcleft += diff;
		rowinset = ( rowinset - diff ) & 3;
		}
	diff = dstrect->rectBottom - clipBounds.rectBottom;
	if ( diff > 0 )
		height  -= diff;
	diff = dstrect->rectRight - clipBounds.rectRight;
	if ( diff > 0 )
		width   -= diff;
	
	// bail if we have nothing to do
	if ( height <= 0
	||   width  <= 0 )
		{
		return;
		}
	
	// get a pointer to the first pixel in the source
	int srcrowbytes = srcimage->GetRowBytes();
	DTSRect srcimageBounds;
	srcimage->GetBounds( &srcimageBounds );
	const uchar * srcbits  = ((uchar *) srcimage->GetBits())
		+ srcrowbytes * ( srctop - srcimageBounds.rectTop )
		+ ( srcleft - srcimageBounds.rectLeft );
	
	// get a pointer to the first pixel in the destination
	int dstrowbytes = dstimage->GetRowBytes();
	DTSRect dstimageBounds;
	dstimage->GetBounds( &dstimageBounds );
	uchar * dstbits = ((uchar *) dstimage->GetBits())
		+ dstrowbytes * ( dsttop - dstimageBounds.rectTop )
		+ ( dstleft - dstimageBounds.rectLeft );
	
	// change rowbytes to offsets
	srcrowbytes -= width;
	dstrowbytes -= width;
	
	// for each row
	for ( ;  height > 0;  --height )
		{
		// for each pixel
		int counter = rowinset;
		for ( diff = width;  diff > 0;  --diff )
			{
			if ( counter & 3 )
				{
				*dstbits = *srcbits;
				}
			++srcbits;
			++dstbits;
			--counter;
			}
		
		// offset to next row
		srcbits += srcrowbytes;
		dstbits += dstrowbytes;
		rowinset ^= 2;
		}
}


/*
**	FastBlitBlend25Transparent()
**
**	draw offscreen transparent faster than the slow versions of quickdraw
**	assume source and destination are both default 8 bit cluts
**	assume no scaling
**	assume no stretching
**	assume source rect is completely within the source image bounds
**	assume we might need to clip the source rect so the destination fits in the 
**		destination image bounds
**	assume a rectangular clipping region smaller than the destination image bounds
*/
void
FastBlitBlend25Transparent( const DTSOffView * view, const DTSImage * srcimage,
	const DTSRect * srcrect, const DTSRect * dstrect )
{
	// pin the destination rectangle to be within the destination image
	// shrink the source rectangle accordingly
	const DTSImage * dstimage = view->offImage;
	DTSRegion cliprgn;
	view->GetMask( &cliprgn );
	int dsttop  = dstrect->rectTop;
	int dstleft = dstrect->rectLeft;
	int srctop  = srcrect->rectTop;
	int srcleft = srcrect->rectLeft;
	int height  = dstrect->rectBottom - dsttop;
	int width   = dstrect->rectRight  - dstleft;
	DTSRect clipBounds;
	cliprgn.GetBounds( &clipBounds );
	int diff = clipBounds.rectTop - dsttop;
	int rowinset = 3;
	if ( diff > 0 )
		{
		height  -= diff;
		dsttop  += diff;
		srctop  += diff;
		if ( diff & 1 )
			rowinset = 1;
		}
	diff = clipBounds.rectLeft - dstleft;
	if ( diff > 0 )
		{
		width   -= diff;
		dstleft += diff;
		srcleft += diff;
		rowinset = ( rowinset - diff ) & 3;
		}
	diff = dstrect->rectBottom - clipBounds.rectBottom;
	if ( diff > 0 )
		height  -= diff;
	diff = dstrect->rectRight - clipBounds.rectRight;
	if ( diff > 0 )
		width   -= diff;
	
	// bail if we have nothing to do
	if ( height <= 0
	||   width  <= 0 )
		{
		return;
		}
	
	// get a pointer to the first pixel in the source
	int srcrowbytes = srcimage->GetRowBytes();
	DTSRect srcimageBounds;
	srcimage->GetBounds( &srcimageBounds );
	const uchar * srcbits  = ((uchar *) srcimage->GetBits())
		+ srcrowbytes * ( srctop - srcimageBounds.rectTop )
		+ ( srcleft - srcimageBounds.rectLeft );
	
	// get a pointer to the first pixel in the destination
	int dstrowbytes = dstimage->GetRowBytes();
	DTSRect dstimageBounds;
	dstimage->GetBounds( &dstimageBounds );
	uchar * dstbits = ((uchar *) dstimage->GetBits())
		+ dstrowbytes * ( dsttop - dstimageBounds.rectTop )
		+ ( dstleft - dstimageBounds.rectLeft );
	
	// change rowbytes to offsets
	srcrowbytes -= width;
	dstrowbytes -= width;
	
	// for each row
	for ( ;  height > 0;  --height )
		{
		// for each pixel
		int counter = rowinset;
		for ( diff = width;  diff > 0;  --diff )
			{
			if ( counter & 3 )
				{
				int pixel = *srcbits;
				if ( pixel )
					*dstbits = pixel;
				}
			++srcbits;
			++dstbits;
			--counter;
			}
		
		// offset to next row
		srcbits += srcrowbytes;
		dstbits += dstrowbytes;
		rowinset ^= 2;
		}
}


/*
**	FastBlitBlend50()
**
**	draw offscreen transparent faster than the slow versions of quickdraw
**	assume source and destination are both default 8 bit cluts
**	assume no scaling
**	assume no stretching
**	assume source rect is completely within the source image bounds
**	assume we might need to clip the source rect so the destination fits in the 
**		destination image bounds
**	assume a rectangular clipping region smaller than the destination image bounds
*/
void
FastBlitBlend50( const DTSOffView * view, const DTSImage * srcimage,
	const DTSRect * srcrect, const DTSRect * dstrect )
{
	// pin the destination rectangle to be within the destination image
	// shrink the source rectangle accordingly
	const DTSImage * dstimage = view->offImage;
	DTSRegion cliprgn;
	view->GetMask( &cliprgn );
	int dsttop  = dstrect->rectTop;
	int dstleft = dstrect->rectLeft;
	int srctop  = srcrect->rectTop;
	int srcleft = srcrect->rectLeft;
	int height  = dstrect->rectBottom - dsttop;
	int width   = dstrect->rectRight  - dstleft;
	DTSRect clipBounds;
	cliprgn.GetBounds( &clipBounds );
	int diff = clipBounds.rectTop - dsttop;
	int skipoddnumrows = 0;
	if ( diff > 0 )
		{
		height  -= diff;
		dsttop  += diff;
		srctop  += diff;
		skipoddnumrows = diff & 1;
		}
	diff = clipBounds.rectLeft - dstleft;
	if ( diff > 0 )
		{
		width   -= diff;
		dstleft += diff;
		srcleft += diff;
		skipoddnumrows ^= diff & 1;
		}
	diff = dstrect->rectBottom - clipBounds.rectBottom;
	if ( diff > 0 )
		height  -= diff;
	
	diff = dstrect->rectRight - clipBounds.rectRight;
	if ( diff > 0 )
		width   -= diff;
	
	// bail if we have nothing to do
	if ( height <= 0
	||   width  <= 0 )
		{
		return;
		}
	
	// get a pointer to the first pixel in the source
	int srcrowbytes = srcimage->GetRowBytes();
	DTSRect srcimageBounds;
	srcimage->GetBounds( &srcimageBounds );
	const uchar * srcbits  = ((uchar *) srcimage->GetBits())
		+ srcrowbytes * ( srctop - srcimageBounds.rectTop )
		+ ( srcleft - srcimageBounds.rectLeft );
	
	// get a pointer to the first pixel in the destination
	int dstrowbytes = dstimage->GetRowBytes();
	DTSRect dstimageBounds;
	dstimage->GetBounds( &dstimageBounds );
	uchar * dstbits = ((uchar *) dstimage->GetBits())
		+ dstrowbytes * ( dsttop - dstimageBounds.rectTop )
		+ ( dstleft - dstimageBounds.rectLeft );
	
	// for each row
	const uchar * srcrow = srcbits;
	uchar * dstrow = dstbits;
	if ( skipoddnumrows )
		goto oddrow;
	
	for(;;)
		{
		// for each pixel
		for ( diff = width;  diff > 0;  diff -= 2 )
			{
			*dstbits = *srcbits;
			srcbits += 2;
			dstbits += 2;
			}
		
		// next row
		--height;
		if ( height <= 0 )
			break;
		
		// offset to next row
		srcrow += srcrowbytes;
		dstrow += dstrowbytes;
		srcbits = srcrow;
		dstbits = dstrow;
oddrow:
		++srcbits;
		++dstbits;
		
		// for each pixel
		for ( diff = width - 1;  diff > 0;  diff -= 2 )
			{
			*dstbits = *srcbits;
			srcbits += 2;
			dstbits += 2;
			}
		
		// next row
		--height;
		if ( height <= 0 )
			break;
		
		// offset to next row
		srcrow += srcrowbytes;
		dstrow += dstrowbytes;
		srcbits = srcrow;
		dstbits = dstrow;
		}
}


/*
**	FastBlitBlend50Transparent()
**
**	draw offscreen transparent faster than the slow versions of quickdraw
**	assume source and destination are both default 8 bit cluts
**	assume no scaling
**	assume no stretching
**	assume source rect is completely within the source image bounds
**	assume we might need to clip the source rect so the destination fits in the 
**		destination image bounds
**	assume a rectangular clipping region smaller than the destination image bounds
*/
void
FastBlitBlend50Transparent( const DTSOffView * view, const DTSImage * srcimage,
	const DTSRect * srcrect, const DTSRect * dstrect )
{
	// pin the destination rectangle to be within the destination image
	// shrink the source rectangle accordingly
	const DTSImage * dstimage = view->offImage;
	DTSRegion cliprgn;
	view->GetMask( &cliprgn );
	int dsttop  = dstrect->rectTop;
	int dstleft = dstrect->rectLeft;
	int srctop  = srcrect->rectTop;
	int srcleft = srcrect->rectLeft;
	int height  = dstrect->rectBottom - dsttop;
	int width   = dstrect->rectRight  - dstleft;
	DTSRect clipBounds;
	cliprgn.GetBounds( &clipBounds );
	int diff = clipBounds.rectTop - dsttop;
	int skipoddnumrows = 0;
	if ( diff > 0 )
		{
		height  -= diff;
		dsttop  += diff;
		srctop  += diff;
		skipoddnumrows = diff & 1;
		}
	diff = clipBounds.rectLeft - dstleft;
	if ( diff > 0 )
		{
		width   -= diff;
		dstleft += diff;
		srcleft += diff;
		skipoddnumrows ^= diff & 1;
		}
	diff = dstrect->rectBottom - clipBounds.rectBottom;
	if ( diff > 0 )
		height  -= diff;
	diff = dstrect->rectRight - clipBounds.rectRight;
	if ( diff > 0 )
		width   -= diff;
	
	// bail if we have nothing to do
	if ( height <= 0
	||   width  <= 0 )
		{
		return;
		}
	
	// get a pointer to the first pixel in the source
	int srcrowbytes = srcimage->GetRowBytes();
	DTSRect srcimageBounds;
	srcimage->GetBounds( &srcimageBounds );
	const uchar * srcbits  = ((uchar *) srcimage->GetBits())
		+ srcrowbytes * ( srctop - srcimageBounds.rectTop )
		+ ( srcleft - srcimageBounds.rectLeft );
	
	// get a pointer to the first pixel in the destination
	int dstrowbytes = dstimage->GetRowBytes();
	DTSRect dstimageBounds;
	dstimage->GetBounds( &dstimageBounds );
	uchar * dstbits = ((uchar *) dstimage->GetBits())
		+ dstrowbytes * ( dsttop - dstimageBounds.rectTop )
		+ ( dstleft - dstimageBounds.rectLeft );
	
	// for each row
	const uchar * srcrow = srcbits;
	uchar * dstrow = dstbits;
	if ( skipoddnumrows )
		goto oddrow;
	
	for(;;)
		{
		// for each pixel
		int pixel;
		for ( diff = width;  diff > 0;  diff -= 2 )
			{
			pixel = *srcbits;
			if ( pixel )
				*dstbits = pixel;
			srcbits += 2;
			dstbits += 2;
			}
		
		// next row
		--height;
		if ( height <= 0 )
			break;
		
		// offset to next row
		srcrow += srcrowbytes;
		dstrow += dstrowbytes;
		srcbits = srcrow;
		dstbits = dstrow;
oddrow:
		++srcbits;
		++dstbits;
		
		// for each pixel
		for ( diff = width - 1;  diff > 0;  diff -= 2 )
			{
			pixel = *srcbits;
			if ( pixel )
				*dstbits = pixel;
			srcbits += 2;
			dstbits += 2;
			}
		
		// next row
		--height;
		if ( height <= 0 )
			break;
		
		// offset to next row
		srcrow += srcrowbytes;
		dstrow += dstrowbytes;
		srcbits = srcrow;
		dstbits = dstrow;
		}
}


/*
**	FastBlitBlend75()
**
**	draw offscreen transparent faster than the slow versions of quickdraw
**	assume source and destination are both default 8 bit cluts
**	assume no scaling
**	assume no stretching
**	assume source rect is completely within the source image bounds
**	assume we might need to clip the source rect so the destination fits in the 
**		destination image bounds
**	assume a rectangular clipping region smaller than the destination image bounds
*/
void
FastBlitBlend75( const DTSOffView * view, const DTSImage * srcimage,
	const DTSRect * srcrect, const DTSRect * dstrect )
{
	// pin the destination rectangle to be within the destination image
	// shrink the source rectangle accordingly
	const DTSImage * dstimage = view->offImage;
	DTSRegion cliprgn;
	view->GetMask( &cliprgn );
	int dsttop  = dstrect->rectTop;
	int dstleft = dstrect->rectLeft;
	int srctop  = srcrect->rectTop;
	int srcleft = srcrect->rectLeft;
	int height  = dstrect->rectBottom - dsttop;
	int width   = dstrect->rectRight  - dstleft;
	DTSRect clipBounds;
	cliprgn.GetBounds( &clipBounds );
	int diff = clipBounds.rectTop - dsttop;
	int rowinset = 3;
	if ( diff > 0 )
		{
		height  -= diff;
		dsttop  += diff;
		srctop  += diff;
		if ( diff & 1 )
			rowinset = 1;
		}
	diff = clipBounds.rectLeft - dstleft;
	if ( diff > 0 )
		{
		width   -= diff;
		dstleft += diff;
		srcleft += diff;
		rowinset = ( rowinset - diff ) & 3;
		}
	diff = dstrect->rectBottom - clipBounds.rectBottom;
	if ( diff > 0 )
		height  -= diff;
	diff = dstrect->rectRight - clipBounds.rectRight;
	if ( diff > 0 )
		width   -= diff;
	
	// bail if we have nothing to do
	if ( height <= 0
	||   width  <= 0 )
		{
		return;
		}
	
	// get a pointer to the first pixel in the source
	int srcrowbytes = srcimage->GetRowBytes();
	DTSRect srcimageBounds;
	srcimage->GetBounds( &srcimageBounds );
	const uchar * srcbits  = ((uchar *) srcimage->GetBits())
		+ srcrowbytes * ( srctop - srcimageBounds.rectTop )
		+ ( srcleft - srcimageBounds.rectLeft );
	
	// get a pointer to the first pixel in the destination
	int dstrowbytes = dstimage->GetRowBytes();
	DTSRect dstimageBounds;
	dstimage->GetBounds( &dstimageBounds );
	uchar * dstbits = ((uchar *) dstimage->GetBits())
		+ dstrowbytes * ( dsttop - dstimageBounds.rectTop )
		+ ( dstleft - dstimageBounds.rectLeft );
	
	// for each row
	const uchar * srcrow = srcbits;
	uchar * dstrow = dstbits;
	for ( ;  height > 0;  --height )
		{
		// inset the row;
		srcbits += rowinset;
		dstbits += rowinset;
		
		// for each pixel
		for ( diff = width - rowinset;  diff > 0;  diff -= 4 )
			{
			*dstbits = *srcbits;
			srcbits += 4;
			dstbits += 4;
			}
		
		// offset to next row
		srcrow += srcrowbytes;
		dstrow += dstrowbytes;
		srcbits = srcrow;
		dstbits = dstrow;
		rowinset ^= 2;
		}
}


/*
**	FastBlitBlend75Transparent()
**
**	draw offscreen transparent faster than the slow versions of quickdraw
**	assume source and destination are both default 8 bit cluts
**	assume no scaling
**	assume no stretching
**	assume source rect is completely within the source image bounds
**	assume we might need to clip the source rect so the destination fits in the 
**		destination image bounds
**	assume a rectangular clipping region smaller than the destination image bounds
*/
void
FastBlitBlend75Transparent( const DTSOffView * view, const DTSImage * srcimage,
	const DTSRect * srcrect, const DTSRect * dstrect )
{
	// pin the destination rectangle to be within the destination image
	// shrink the source rectangle accordingly
	const DTSImage * dstimage = view->offImage;
	DTSRegion cliprgn;
	view->GetMask( &cliprgn );
	int dsttop  = dstrect->rectTop;
	int dstleft = dstrect->rectLeft;
	int srctop  = srcrect->rectTop;
	int srcleft = srcrect->rectLeft;
	int height  = dstrect->rectBottom - dsttop;
	int width   = dstrect->rectRight  - dstleft;
	DTSRect clipBounds;
	cliprgn.GetBounds( &clipBounds );
	int diff = clipBounds.rectTop - dsttop;
	int rowinset = 3;
	if ( diff > 0 )
		{
		height  -= diff;
		dsttop  += diff;
		srctop  += diff;
		if ( diff & 1 )
			rowinset = 1;
		}
	diff = clipBounds.rectLeft - dstleft;
	if ( diff > 0 )
		{
		width   -= diff;
		dstleft += diff;
		srcleft += diff;
		rowinset = ( rowinset - diff ) & 3;
		}
	diff = dstrect->rectBottom - clipBounds.rectBottom;
	if ( diff > 0 )
		height  -= diff;
	diff = dstrect->rectRight - clipBounds.rectRight;
	if ( diff > 0 )
		width   -= diff;
	
	// bail if we have nothing to do
	if ( height <= 0
	||   width  <= 0 )
		{
		return;
		}
	
	// get a pointer to the first pixel in the source
	int srcrowbytes = srcimage->GetRowBytes();
	DTSRect srcimageBounds;
	srcimage->GetBounds( &srcimageBounds );
	const uchar * srcbits  = ((uchar *) srcimage->GetBits())
		+ srcrowbytes * ( srctop - srcimageBounds.rectTop )
		+ ( srcleft - srcimageBounds.rectLeft );
	
	// get a pointer to the first pixel in the destination
	int dstrowbytes = dstimage->GetRowBytes();
	DTSRect dstimageBounds;
	dstimage->GetBounds( &dstimageBounds );
	uchar * dstbits = ((uchar *) dstimage->GetBits())
		+ dstrowbytes * ( dsttop - dstimageBounds.rectTop )
		+ ( dstleft - dstimageBounds.rectLeft );
	
	// for each row
	const uchar * srcrow = srcbits;
	uchar * dstrow = dstbits;
	for ( ;  height > 0;  --height )
		{
		// inset the row;
		srcbits += rowinset;
		dstbits += rowinset;
		
		// for each pixel
		for ( diff = width - rowinset;  diff > 0;  diff -= 4 )
			{
			int pixel = *srcbits;
			if ( pixel )
				*dstbits = pixel;
			srcbits += 4;
			dstbits += 4;
			}
		
		// offset to next row
		srcrow += srcrowbytes;
		dstrow += dstrowbytes;
		srcbits = srcrow;
		dstbits = dstrow;
		rowinset ^= 2;
		}
}


