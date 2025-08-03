/*
**	Shadows_cl.h			Clan Lord Client
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

#ifndef SHADOWS_CL_H
#define SHADOWS_CL_H

#include "ClanLord.h"


//	tell the shadowcasters where the sun is supposed to be.
//	shadowLevel is 0..100; controls how dark the shadows are
//		[internally quantized to 25% increments]
//  sunAngle  is azimuth, -1..360 degrees, counterclockwise from east
//  declination is -90 to 90, south pole to north pole.
//		[Not currently used]

void	SetShadowAngle( int shadowLevel, int sunAngle, int declination );
int		GetShadowLevel();


// Values of the 'flags' param to CreateShadow()
// Maybe one day they could be incorporated into the true PictDef flags.
// (the values are deliberately chosen not to conflict with any currently-defined flag bits)
enum
{
	// some images just won't have any shadows
	kPictDefShadowNone		= 0,
	
	// this image is "upstanding": so its shadow is a rotated copy of the source image bits
	kPictDefShadowNormal	= 1 << 3,	// 0x08
	
	// a drop shadow is a straight black&white rendition of the source, offset a few pixels
	kPictDefShadowDrop		= 1 << 4,	// 0x10,
	
	// blurry ovals for images that are otherwise intractable
	kPictDefShadowOval		= 1 << 5,	// 0x20,
	
	
	kPictDefShadowMask		= kPictDefShadowNormal | kPictDefShadowDrop | kPictDefShadowOval
};


// Draw the shadow for the given image
#ifdef USE_OPENGL
void	CreateShadow( DTSOffView * view, ImageCache * cache,
			const DTSRect * srcRect, const DTSRect * dstRect, uint flags );
#else
void	CreateShadow( DTSOffView * view, const DTSImage * img,
			const DTSRect * srcRect, const DTSRect * dstRect, uint flags );
#endif	// USE_OPENGL

// determine what pose to cast the shadow with, given the sun's angle
int		ChooseShadowPose( int state );


#endif	// SHADOWS_CL_H
