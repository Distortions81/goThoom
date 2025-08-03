/*
**	Shadows_cl.cp		Clan Lord Client
**
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
#include "Shadows_cl.h"

#ifdef USE_OPENGL
# include "OpenGL_cl.h"
#endif	// USE_OPENGL


//
// internal definitions
//

// for oval and drop-shadows, these are the (maximum) radii of displacement
// * vary these based on image dimensions.
const int kShadowHOffset = 5;
const int kShadowVOffset = 5;


// #define SHADOW_TEST

//
// internal variables
//
#ifdef SHADOW_TEST
static int	gShadowLevel	= 100;		// 0, 25, 50, 75, 100: none to darkest shadow
static int	gSunAngle		= 270;		// azimuth of sun 0-360 ccw from east

static float sShadowSine	= -1.0F;	// cached trig results
static float sShadowCosine	= 0.0F;		// start 'em at 270 degrees
#else
static int	gShadowLevel	= 50;		// 0, 25, 50, 75, 100: none to darkest shadow
static int	gSunAngle		= 90;		// azimuth of sun 0-360 ccw from east

static float sShadowSine	= 1.0F;		// cached trig results
static float sShadowCosine	= 0.0F;		// start 'em at 90 degrees
#endif
static int	gSunDeclination	= 45;		// sun's declination, -90 to 90, south pole to north pole
										// (currently not used in the code)
#ifdef USE_OPENGL
const float gLocalLatitude = -35.0f;	// neg is south of the equator (the shadows fall south
										// of the mobiles, so the sun must be north of us)
										// some GM will have to officiate this
//float gLocalSunDeclination = 45.0f;	// 45 degrees up from the horizon implies...
										// ...that the shadows is the same length as...
										// ...the object is tall:
static float gLocalSunDeclinationScale = 1.0f;
#endif	// USE_OPENGL


//
// internal routines
//
static void CastShadowRotate( const DTSImage * image, DTSImage * shadow,
	const DTSRect * srcRect, DTSRect * dstRect );
static void CreateDropShadow( const DTSImage * image, DTSImage * shadow,
	const DTSRect * srcRect, DTSRect * dstRect );
static void CreateOvalShadow( const DTSImage * image, DTSImage * shadow,
	const DTSRect * srcRect, DTSRect * dstRect );

inline static Fixed FixedMul( Fixed a, Fixed b );


/*
**	GetShadowLevel()
**
**	minimal accessor function
*/
int
GetShadowLevel()
{
	return gShadowLevel;
}


/*
**	SetShadowAngle()
**
**	cache the shadowcaster state when the night spell informs us of a change
*/
void
SetShadowAngle( int shadowLevel, int sunAngle, int declination )
{
	if ( shadowLevel >= 0 )
		gShadowLevel = shadowLevel;
	
	// keep angle to 0-360
	while ( sunAngle > 360 )
		sunAngle -= 360;
	while ( sunAngle < 0 )
		sunAngle += 360;
	
	gSunAngle = sunAngle;
//	ShowMessage( "angle: %d", sunAngle );
	
	gSunDeclination = declination;
#ifdef USE_OPENGL
#if 1		// turn this off to revert to the previous shadow lengths
		// until the server is reved to send the real declination,
		// we can use a simple linear estimate
	float sunAngle2 = sunAngle;
	if ( sunAngle2 > 180.0f )
		sunAngle2 -= 180.0f;				// quadrants 3 and 4 behave like quadrants 1 and 2
	if ( sunAngle2 > 90.0f )
		sunAngle2 = 180.0f - sunAngle2;		// quadrant 2 behaves like the mirror of quadrant 1
	
	sunAngle2 /= 90.0f;										// normalize 0-90 into 0-1
	sunAngle2 *= (90.0f - std::fabs(gLocalLatitude));		// at noon, the maximum sun angle is
															// 90 (directly overhead) - latitude
	
	// the CL shadow algorithm isn't going to handle edge cases too well, so let's cheat
	// recall that horizon is 0, directly overhead is 90
	
	// INCREASE this to shorten the morning/evening shadows
	const float kMinimumLocalSunDeclination =  7.0f;
	
	// DECREASE this to lengthen the midday shadows
	const float kMaximumLocalSunDeclination = 85.0f;
	
	if ( sunAngle2 < kMinimumLocalSunDeclination )
		sunAngle2 = kMinimumLocalSunDeclination;
	if ( sunAngle2 > kMaximumLocalSunDeclination )
		sunAngle2 = kMaximumLocalSunDeclination;
	
	// cotangent = tangent(90 - angle)
	gLocalSunDeclinationScale = tanf( (90.0f - sunAngle2) * float(pi) / 180.0f );
#endif
#endif	// USE_OPENGL
	
	// do some simple trig
	float rAngle = (pi / 180.0) * sunAngle;	// degrees-to-radians
	
	sShadowSine   = sin( rAngle );
	sShadowCosine = cos( rAngle );
	
	// we use the above two statics for the rotating shadowcaster
	// since it requires as inputs sin(a+90) and cos(a+90)
	// which, conveniently, are equal to cos(a) and -sin(a)
}


/*
	NOTE This is all rather preliminary and doesn't work especially well on a large
	class of images. It works best on mobiles, and on images which are viewed from
	nearly horizontal. To the extent that the image's vertical dimension is a good
	analog of the represented thing's Z dimension, the shadows are fairly convincing.
	Objects depicted as if viewed from the top, or which have a great amount of
	Y-axis information mingled in with the Z-axis, tend to look like cardboard
	cutouts standing on edge.
*/


/*
**	CreateShadow()
**
**	This is the principal entry point to the shadow engine.
**	call this routine with a pointer to the CLOffview, the source image,
**	the boxes defining the input and output regions, and some flags, and
**	it chooses the appropriate type of shadow to cast. Then it draws it.
*/
void
CreateShadow( DTSOffView * view,
#ifdef USE_OPENGL
			ImageCache * cache,
#else
			const DTSImage * image,
#endif	// USE_OPENGL
			const DTSRect * inSrcRect,
			const DTSRect * inDstRect,
			uint flags )
{
	// bail if there's no shadows in the area
	if ( gShadowLevel == 0 )
		return;
	
	// bail if the player doesn't want shadows
	if ( gPrefsData.pdMaxNightPct == 0 )
		return;
	
	int hOffset = -kShadowHOffset * sShadowCosine;
	int vOffset = kShadowVOffset * sShadowSine;
	
	DTSImage shadow;
	DTSRect dstRect = *inDstRect;
	DTSRect srcRect = *inSrcRect;
	
#ifdef USE_OPENGL
	const DTSImage * image = &cache->icImage.cliImage;
	float shadowLevel = float( gShadowLevel ) / 100.0F;
	TextureObject * tObj = cache->textureObject;
	
	// is OGL shadowing permitted?
	bool bCanShadow = gUsingOpenGL
					&& gOpenGLEnableEffects
					&& (not gOGLDisableShadowEffects)
					&& tObj != nullptr;
#endif	// USE_OPENGL
	
	// choose the appropriate shadow maker function
	switch ( flags & kPictDefShadowMask )
		{
		case kPictDefShadowNone:
			// some images just won't have any shadows
			return;
		
		case kPictDefShadowNormal:
			// the image is "upstanding": shadow is a rotated copy
			// of the source image bits
#ifdef USE_OPENGL
			if ( bCanShadow
			&&   tObj->drawAsRotatedShadow( srcRect, dstRect,
					gSunAngle, gLocalSunDeclinationScale, shadowLevel, cache ) )
				{
				return;
				}
#endif	// USE_OPENGL
			CastShadowRotate( image, &shadow, &srcRect, &dstRect );
			break;
		
		case kPictDefShadowDrop:
			// a drop shadow is a straight black+white rendition of the source
			// but just offset a few pixels
#ifdef USE_OPENGL
			if ( bCanShadow
			&& 	 tObj->drawAsDropShadow( srcRect, dstRect, hOffset, vOffset,
					shadowLevel, cache ) )
				{
				return;
				}
#endif	// USE_OPENGL
			CreateDropShadow( image, &shadow, &srcRect, &dstRect );
			dstRect.Offset( hOffset, vOffset );
			break;
		
		case kPictDefShadowOval:
			// blurry ovals for images that are otherwise intractable
#ifdef USE_OPENGL
// note that we are reusing dropshadow, cuz i don't think oval is being used
			if ( bCanShadow
			&&   tObj->drawAsDropShadow( srcRect, dstRect, hOffset, vOffset,
					shadowLevel, cache ) )
				{
				return;
				}
#endif	// USE_OPENGL
			CreateOvalShadow( image, &shadow, &srcRect, &dstRect );
			dstRect.Offset( hOffset, vOffset );
			break;
		}
	
	// the shadow image is now complete
	// blast the bits into the offview
	DTSRect shadowBounds;
	shadow.GetBounds( &shadowBounds );
	switch ( gShadowLevel )
		{
		case 25:
#ifdef USE_OPENGL
			if ( gUsingOpenGL )
				drawOGLPixmap( shadowBounds, dstRect, shadow.GetRowBytes(),
					kIndexToAlphaMapAlpha25TransparentZero, shadow.GetBits() );
			else
#endif	// USE_OPENGL
				QualityBlitBlend75Transparent( view, &shadow, &shadowBounds, &dstRect );
			break;
		
		case 50:
#ifdef USE_OPENGL
			if ( gUsingOpenGL )
				drawOGLPixmap( shadowBounds, dstRect, shadow.GetRowBytes(),
					kIndexToAlphaMapAlpha50TransparentZero, shadow.GetBits() );
			else
#endif	// USE_OPENGL
				QualityBlitBlend50Transparent( view, &shadow, &shadowBounds, &dstRect );
			break;
		
		case 75:
#ifdef USE_OPENGL
			if ( gUsingOpenGL )
				drawOGLPixmap( shadowBounds, dstRect, shadow.GetRowBytes(),
					kIndexToAlphaMapAlpha75TransparentZero, shadow.GetBits() );
			else
#endif	// USE_OPENGL
				QualityBlitBlend25Transparent( view, &shadow, &shadowBounds, &dstRect );
			break;
		
		case 100:
#ifdef USE_OPENGL
			if ( gUsingOpenGL )
				drawOGLPixmap( shadowBounds, dstRect, shadow.GetRowBytes(),
					kIndexToAlphaMapAlpha100TransparentZero, shadow.GetBits() );
			else
#endif	// USE_OPENGL
				BlitTransparent( view, &shadow, &shadowBounds, &dstRect );
			break;
		}
	
	shadow.DisposeBits();
}


/*
**	CreateDropShadow()
**
**	just duplicate the non-transparent pixels
**	then draw them, offset a bit
*/
void
CreateDropShadow( const DTSImage * image, DTSImage * shadow,
	const DTSRect * srcRect, DTSRect * /* dstRect */ )
{
	// experimental for intermediate colors...
//	RGBColor temp = {0x7FFF, 0x100, 0x100};
//	uchar cIndex = (uchar) Color2Index(&temp);
	
	int width =  srcRect->rectRight - srcRect->rectLeft;
	int height = srcRect->rectBottom - srcRect->rectTop;
	
	if ( noErr != shadow->Init( nullptr, width, height, 8 ) )
		return;
	if ( noErr != shadow->AllocateBits() )
		return;
	
	int srcRowBytes = image->GetRowBytes();
	int dstRowBytes = shadow->GetRowBytes();
	bzero( shadow->GetBits(), dstRowBytes * height );
	
	const uchar * src = static_cast<uchar *>( image->GetBits() );
	uchar * dst = static_cast<uchar *>( shadow->GetBits() );
	
	src += srcRowBytes * srcRect->rectTop + srcRect->rectLeft;
	
	srcRowBytes -= width;
	dstRowBytes -= width;
	
	for ( int row = height; row > 0; --row )
		{
		for ( int col = width; col > 0; --col )
			{
			if ( *src++ )
				{
				*dst = 0x0FF; /* cIndex; */
				}
			++dst;
			}
		src += srcRowBytes;
		dst += dstRowBytes;
		}
	
//	dstRect->Offset( sizeFactor * hOffset, sizeFactor * vOffset );
}



/*
**	CreateOvalShadow()
**
**	make a fuzzy blurry black oval somehow
*/
void
CreateOvalShadow( const DTSImage * /* image */, DTSImage * shadow,
	const DTSRect * srcRect, DTSRect * /* dstRect */ )
{
	int srcWidth = srcRect->rectRight - srcRect->rectLeft;
	int srcHeight = srcRect->rectBottom - srcRect->rectTop;
	
	if ( noErr != shadow->Init( nullptr, srcWidth, srcHeight, 8 ) )
		return;
	if ( noErr != shadow->AllocateBits() )
		return;
	
	uchar * dst = static_cast<uchar *>( shadow->GetBits() );
	int dstRowBytes = shadow->GetRowBytes();
	bzero( dst, dstRowBytes * srcHeight );
	
	dstRowBytes -= srcWidth;
	
	// more to follow
}



/*
**	FixedMul()
**
**	a much faster FixMul
**	but is it accurate enough?
*/
inline static Fixed
FixedMul( Fixed a, Fixed b )
{
	return ( a >> 8 ) * ( b >> 8 );
}


/*
**	CastShadowRotate()
**
**	create a pixmap that's a free rotation of the input mobile image
**	with black bits everywhere the source image is not white
**	the angle is controlled by the file static vars sShadowSine & ~Cosine
*/
void
CastShadowRotate( const DTSImage * image, DTSImage * shadow,
	const DTSRect * srcRect, DTSRect * dstRect )
{
	int srcWidth  = srcRect->rectRight  - srcRect->rectLeft;
	int srcHeight = srcRect->rectBottom - srcRect->rectTop;
	
	// figure bounding box of rotated pixel map
	int shadowWidth, shadowHeight;
	if ( sShadowSine * sShadowCosine >= 0 )
		{
		shadowWidth = - srcHeight * sShadowCosine - srcWidth * sShadowSine;
		shadowHeight =  srcHeight * sShadowSine   + srcWidth * sShadowCosine;
		}
	else
		{
		shadowWidth = - srcHeight * sShadowCosine + srcWidth * sShadowSine;
		shadowHeight =  srcHeight * sShadowSine   - srcWidth * sShadowCosine;
		}
	
	// beware of wrong signs
	if ( shadowWidth < 0 )
		shadowWidth = -shadowWidth;
	if ( shadowHeight < 0 )
		shadowHeight = -shadowHeight;
	
	// now we know its dimensions
	// create shadow bitmap
	if ( noErr != shadow->Init( nullptr, shadowWidth, shadowHeight, 8 ) )
		return;
	if ( noErr != shadow->AllocateBits() )
		return;
	
	int dstRowBytes = shadow->GetRowBytes();
	int srcRowBytes = image->GetRowBytes();
	
	bzero( shadow->GetBits(), dstRowBytes * shadowHeight );
	uchar * dst = static_cast<uchar *>( shadow->GetBits() );
	const uchar * src = static_cast<uchar *>( image->GetBits() );
#if DEBUG_VERSION
	const uchar * limit = dst + (dstRowBytes * shadowHeight);
#endif
	
	// find last non-empty row of source
	// (I wish I could cache this in the pdef somehow...)
	const uchar * bottomrow = src + srcRowBytes * srcHeight;
	int rowToSkip = 0;
	for ( int vert = srcHeight; vert; --vert )
		{
		for ( int horz = srcWidth; horz; --horz )
			{
			if ( *bottomrow )
				{
				// found a non-empty pixel
				rowToSkip = srcHeight - vert;
				goto FoundBottom;
				}
			--bottomrow;
			}
		bottomrow -= srcRowBytes - srcWidth;
		}
FoundBottom:
	
	Fixed topLeftV = 0x08000 - (shadowHeight << 15);
	Fixed topLeftH = 0x08000 - (shadowWidth  << 15);
	
	// remember our rotations are 90 degrees off, and, CCW
	Fixed fxSin = static_cast<Fixed>( int( sShadowCosine * -65536.0F ) );
	Fixed fxCos = static_cast<Fixed>( int( sShadowSine * -65536.0F ) );
	
	Fixed rowH = - FixedMul( topLeftV, fxSin );
	rowH -= FixedMul( topLeftH, fxCos );
	rowH += ( srcRect->rectLeft + srcRect->rectRight ) << 15;
		// includes a DIV 2 to get the center
	
	Fixed rowV = FixedMul( topLeftV, fxCos );
	rowV -= FixedMul( topLeftH, fxSin );
	rowV += ( srcRect->rectTop + srcRect->rectBottom ) << 15;
	
	// pt is the point in the src that corresponds to topLeft
	// not counting the fact that the origin of rotation isn't 0,0
	int ptV = rowV >> 16;
	int ptH = rowH >> 16;
	
	// which we now proceed to fix
	src += ( ptV * srcRowBytes ) + ptH;
	
	for ( int vert = shadowHeight; vert; --vert )
		{
		uchar * dstAddr = dst;
		const uchar * srcAddr = src;
		
		// start point of this row
		Fixed locV = rowV;
		Fixed locH = rowH;
		
		for ( int horz = shadowWidth; horz; --horz )
			{
			ptV = locV >> 16;
			ptH = locH >> 16;
			
			// ignore dest pixels that don't map to valid source bits
			if ( ptV >= srcRect->rectTop
			&&	 ptH >= srcRect->rectLeft
			&&	 ptV <  srcRect->rectBottom
			&&   ptH <  srcRect->rectRight )
				{
				
#if DEBUG_VERSION
				if ( dstAddr < shadow->GetBits() )
					VDebugStr( "underflow in rotate" );
				else
				if ( dstAddr > limit )
					VDebugStr( "Overflow in rotate" );
#endif
				
				if ( *srcAddr )
					*dstAddr = 0x0FF;
				}
			++dstAddr;
			
			// get next source pixel
			locV -= fxSin;
			locH -= fxCos;
			
			ptV -= locV >> 16;
			ptH -= locH >> 16;
			
			// some timmer magic I don't understand yet
			if ( ptV + 1 == 0 )
				srcAddr += srcRowBytes;
			else
			if ( ptV - 1 == 0 )
				srcAddr -= srcRowBytes;
			srcAddr -= ptH;
			}
		
		// get next source pixel for new row
		dst += dstRowBytes;
		
		ptV = rowV >> 16;
		ptH = rowH >> 16;
		
		rowV += fxCos;
		rowH -= fxSin;
		
		ptV -= rowV >> 16;
		ptH -= rowH >> 16;
		
		// more magic
		if ( ptV + 1 == 0 )
			src += srcRowBytes;
		else
		if ( ptV - 1 == 0 )
			src -= srcRowBytes;
		src -= ptH;
		}
	
	// at this point the shadow map is complete and rotated
	// now we must shift it so that its feet coincide
	// with the feet of the input image
	
	dstRect->Size( shadowWidth, shadowHeight );
	
	// fudge factor for sun angles near 45 or 135
	// it helps tuck the shadows upward, rotating around the centerline
	// instead of the baseline
	const float fudge = 1.2F;
	dstRect->Offset(
		(srcWidth / 2) - shadowWidth * (1 + sShadowCosine) / 2,
		srcHeight - rowToSkip - (shadowHeight / 2) * (fudge - sShadowSine)
		);
}

#pragma mark -


/*
**	ChooseShadowPose()
**
**	figure out which pose to use for shadows for a given combination of
**	actual pose and sun angle. Also returns -1 if this pose should never
**	cast a shadow at all.
*/
int
ChooseShadowPose( int state )
{
	// for standing-up mobiles, adjust
	if ( state < kPoseDead )
		{
		int facingAngle = state / 4;					// 0..7 -- e to ne, cw
		int sunAngle = (gSunAngle + 23) / 45;			// 0..7 -- e to se, ccw
		
		// don't ask me about this line please
		int newState = ( facingAngle + sunAngle + 6 ) & 7;
		
		// mix the new state with the sub-pose
		return (newState << 2) + (state & 3);
		}
	
	if ( kPoseDead == state
	||	 kPoseLie  == state )
		{
		// prone & supine cast no shadow
		return -1;		// ** fix this: need to force a drop-shadow instead
		}
	
	// for sit, kneel, etc., just use the actual pose
	return state;
}


