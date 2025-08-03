/*
**	OpenGL_cl.cp		Clanlord Client
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
#include "OpenGL_cl.h"

#include <AGL/agl.h>
#include <AGL/aglRenderers.h>

	// these aren't in aglRenderers.h, but ought to be (they're in CGLRenderers.h)
#ifndef AGL_RENDERER_ATI_RADEON_9700_ID		// ... but who knows; they might show up one day
			// #define kCGLRendererATIRadeon9700ID 0x00021800
#	define AGL_RENDERER_ATI_RADEON_9700_ID		0x00021800
#endif	// AGL_RENDERER_ATI_RADEON_9700_ID
#ifndef AGL_RENDERER_NVIDIA_GEFORCE_FX_ID
			// #define kCGLRendererGeForceFXID	 0x00022400
#	define AGL_RENDERER_NVIDIA_GEFORCE_FX_ID	0x00022400
#endif	// AGL_RENDERER_NVIDIA_GEFORCE_FX_ID

//#include <GL/gl.h>

#include <OpenGL/glu.h>


// borrowed from GameWin_cl.cp
extern	Layout			gLayout;
extern	DTSPoint		gFieldCenter;

GWorldPtr gLightmapGWorld = nullptr;
GWorldPtr gAllBlackGWorld = nullptr;
int gLightmapRadiusLimit = kLightmapMaximumRadius;

static bool gOGLHaveVersion_1_2 = false;
static bool gOGLHaveVersion_1_3 = false;
static bool gOGLHaveVersion_1_4 = false;
static bool gOGLPixelZoomIsEnabled = false;

bool gOpenGLAvailable = false;
bool gUsingOpenGL = false;
bool gOpenGLEnable = false;
bool gOpenGLEnableEffects = true;
bool gOpenGLForceGenericRenderer = false;
bool gOpenGLPermitAnyRenderer = false;
bool gOGLDisableShadowEffects = false;
bool gOGLBlendingIsEnabled = false;
bool gOGLAlphaTestIsEnabled = false;
bool gOGLStencilBufferAvailable = false;
bool gOGLStencilTestIsEnabled = false;
GLuint gOGLCurrentEnabledTextureTarget = 0;
GLfloat gOGLTextureEnvironmentMode = -1;	// not a valid value
GLuint gOGLCurrentTexture = 0;
GLenum gOGLBlendEquationMode = GL_FUNC_ADD;
GLushort *	gIndexToRedMap = nullptr;
GLushort *	gIndexToGreenMap = nullptr;
GLushort *	gIndexToBlueMap = nullptr;
GLint gPixelMapSize = 0;
IndexToAlphaMapArray IndexToAlphaMap[ kIndexToAlphaMapCount ];
IndexToAlphaMapID gOGLCurrentIndexToAlphaMap = kIndexToAlphaMapCount;
	// kIndexToAlphaMapCount isn't a valid id, forcing a mismatch on the first compare

GLuint gOGLSelectionList = 0;	// zero is not a valid display list
bool gOGLUseDither = false;

GLint gOGLNightTexSize = 0;
GLfloat gOGLNightTexHScale = 0.0f;
GLfloat gOGLNightTexVScale = 0.0f;
GLuint gOGLNightTexture = 0;				// zero is not a valid object name
GLclampf gOGLNightTexturePriority = -1.0f;	// -1 is not a valid priority
static GLint gOGLNightTextureFilter = 0;	// 0 is neither GL_NEAREST nor GL_LINEAR
GLuint gLightList = 0;						// zero is not a valid display list
int gNumLightCasters = 0;
int gNumDarkCasters = 0;
bool gUseLightMap = true;

#ifdef OGL_USE_VERTEX_ARRAYS
const GLvoid *	gOGLVertexArray = nullptr;
const GLvoid *	gOGLTexCoordArray = nullptr;
#endif	// OGL_USE_VERTEX_ARRAYS

static bool gOGLHaveTexRectExtension = false;

	// ought to be false, but this is actually safer
static bool gOGLHavePalettedTextureExtension = true;
static bool gOGLHaveClampToEdge = false;
static bool gOGLHaveBGRA = false;
static bool gOGLHavePackedPixels = false;
bool gOGLHaveBlendSubtract = false;				// can we do dest - source?
	// core function in 1.4 or if GL_ARB_imaging
static bool gOGLHaveBlendSubtractCoreFunction = false;
static bool gOGLHaveBlendSubtractExtension = false;		// GL_EXT_blend_subtract
glBlendEquationProcPtr gOGLglBlendEquation = nullptr;

GLuint gOGLTexTarget2D = GL_TEXTURE_2D;
static GLuint gOGLProxyTexTarget2D = GL_PROXY_TEXTURE_2D;

#ifdef DEBUG_VERSION
uint gLargestTextureDimension = 0;
uint gLargestTexturePixels = 0;
#endif	// DEBUG_VERSION

const char * gOGLRendererString = nullptr;
#ifdef OGL_SHOW_DRAWTIME
GLint gOGLRendererID = 0;
#else
static GLint gOGLRendererID = 0;
#endif	// ! OGL_SHOW_DRAWTIME
static bool gOGLRendererRage128 = false;
static bool gOGLRendererGeneric = false;
bool gOGLManageTexturePriority = false;
static bool gOGLBrokenTexSubImage_ColorIndex = false;

static bool gOGLRendererSmallTextureMemory = false;

//long gForceSoftwareOGLRenderer = 0;
//long gAllowNoncompliantOGLRenderer = 0;
AGLContext ctx = nullptr;

#if defined(CARBON_EVENT_TIMER_EXPERIMENT) && defined(USE_OPENGL)
	// just some reasonable defaults
static int gNormalSleep = 20;
static int gInGameSleep = 6;
static int gFFWDSleep = gInGameSleep;
#endif  // defined(CARBON_EVENT_TIMER_EXPERIMENT) && defined(USE_OPENGL)


// crazy wild-eyed mad science experiment with a resizable game window,
// via a sort of "pixel doubling".  Probably won't ever see the light of day.
# define RESIZING_EXPERIMENT		0

#if RESIZING_EXPERIMENT
static float gScaleFactor;
#endif


/*
**	DShowMessage()
*/
#if 0
# define DShowMessage	ShowMessage
#else
# define DShowMessage(...)
#endif


/*
**	isOpenGLAvailable()
**	test for existence via weak-link check
*/
bool
isOpenGLAvailable()
{
	gOpenGLAvailable = ( &aglGetVersion != nullptr );
	
	if ( gOpenGLAvailable )
		{
		GLint major, minor;
		aglGetVersion( &major, &minor );
#ifdef DEBUG_VERSION
		DShowMessage( "OpenGL is available" );
		DShowMessage( "\tAGL Library version == %d.%d", int(major), int(minor) );
#endif
		}
#ifdef DEBUG_VERSION
	else
		{
		DShowMessage( "OpenGL is NOT available" );
		}
#endif
	
	return gOpenGLAvailable;
}


/*
**	HaveExtension()
**	test whether a given OpenGL extension is available
**	source: website: <forgotten>
*/
bool
HaveExtension( const char * extName )
{
	/*
	Search for extName in the extensions string.  Use of strstr()
	is not sufficient because extension names can be prefixes of
	other extension names.  Could use strtok() but the constant
	string returned by glGetString can be in read-only memory.
	
	Also note that glGetString(GL_EXTENSIONS) or QueryExtension
	must be called after the GL context has been bound to a renderer.
	For example, when using GLX it's not until glMakeCurrent is
	issued that we can ask the renderer what extensions are supported. 
	*/

	const char * p = reinterpret_cast<const char *>( glGetString( GL_EXTENSIONS ) );
	size_t extNameLen = strlen( extName );
	const char * end = p + strlen( p );
	
	while ( p < end )
		{
		size_t n = strcspn( p, " " );
		if ( extNameLen == n
		&&   strncmp( extName, p, n ) == 0 )
			{
			return true;
			}
		p += n + 1;
		}
	return false;
}


/*
**	DisplayAGLErrors()
*/
bool
DisplayAGLErrors( const char * file, int line )
{
#ifndef DEBUG_VERSION
# pragma unused( file, line )
#endif

	bool result = false;
	GLenum err;
	while ( (err = aglGetError()) != AGL_NO_ERROR )
		{
#ifdef DEBUG_VERSION
		ShowMessage( "%s:%d aglGetError %d - %s", file, line, int(err), aglErrorString( err ) );
#endif
		result = true;
		}
	return result;
}


/*
**	DisplayOpenGLErrors()
*/
bool
DisplayOpenGLErrors( const char * file, int line )
{
#ifndef DEBUG_VERSION
# pragma unused( file, line )
#endif

	bool result = false;
	GLenum err;
	while ( (err = aglGetError()) != AGL_NO_ERROR )
		{
#ifdef DEBUG_VERSION
		ShowMessage( "%s:%d aglGetError %d - %s", file, line, int(err), aglErrorString(err) );
#endif
		result = true;
		}
	while ( (err = glGetError()) != GL_NO_ERROR )
		{
#ifdef DEBUG_VERSION
		ShowMessage( "%s:%d glGetError %d - %s", file, line, int(err), gluErrorString(err) );
#endif
		result = true;
		}
	return result;
}


/*
**	setOGLState()
*/
void
setOGLState()
{
	bool canUseOpenGL = gOpenGLAvailable && ctx && gOpenGLEnable;
	if ( not canUseOpenGL )
		{
		if ( gUsingOpenGL )
			{
			gUsingOpenGL = false;
			if ( ctx )
				setupOpenGLGameWindow( ctx, gUsingOpenGL );
			}
		}
	else
		{
		if ( not gUsingOpenGL )
			{
			gUsingOpenGL = true;
			setupOpenGLGameWindow( ctx, gUsingOpenGL );
			}
		}
}


/*
**	populatePixelAttributeArray()
*/
static void
populatePixelAttributeArray( const PixelFormatAttributes& requested, GLint attrib[] )
{
	int index = 0;
	attrib[ index++ ] = AGL_RGBA;
	attrib[ index++ ] = AGL_DOUBLEBUFFER;
	
	if ( requested.useRGBColorSize )
		{
			// we probably only need to set one of the components,
			// but it's harmless to set them all
		attrib[ index++ ] = AGL_RED_SIZE;
		attrib[ index++ ] = requested.rgbColorSize;
		attrib[ index++ ] = AGL_GREEN_SIZE;
		attrib[ index++ ] = requested.rgbColorSize;
		attrib[ index++ ] = AGL_BLUE_SIZE;
		attrib[ index++ ] = requested.rgbColorSize;
		}
	
	if ( requested.useStencilSize )
		{
		attrib[ index++ ] = AGL_STENCIL_SIZE;
		attrib[ index++ ] = requested.stencilSize;
		}
	
	if ( requested.useRendererID )
		{
		attrib[ index++ ] = AGL_RENDERER_ID;
		attrib[ index++ ] = requested.rendererID;
		}
	
	if ( requested.permitAnyRenderer )
		attrib[ index++ ] = AGL_ALL_RENDERERS;
	
	if ( requested.useNoRecovery )
		attrib[ index++ ] = AGL_NO_RECOVERY;
	
	if ( requested.useAccelerated )
		attrib[ index++ ] = AGL_ACCELERATED;
	
	attrib[ index++ ] = AGL_NONE;
	
	if ( index > kOGLMaxAttribArraySize )
		ShowMessage( "MAYDAY: array overrun (kOGLMaxAttribArraySize exceeded)" );
}


/*
**	SetOGLDrawable()
*/
static inline GLboolean
SetOGLDrawable()
{
	WindowRef win = DTS2Mac( gGameWindow );
	
#if MAC_OS_X_VERSION_MIN_REQUIRED > 1040
	return aglSetWindowRef( ctx, win );
#else  // 10.4
	return aglSetDrawable( ctx, GetWindowPort( win ) );
#endif  // 10.4
}

/*
**	attachOGLContextToGameWindow()
*/
void
attachOGLContextToGameWindow()
{
	if ( not aglConfigure( AGL_FORMAT_CACHE_SIZE, 1 ) )	// minimize memory
		{
		ShowMessage( "aglConfigure(AGL_FORMAT_CACHE_SIZE) returned error:" );
		ShowAGLErrors();
		}
	
	PixelFormatAttributes requested = {
		true,							// useRGBColorSize
		8,								// rgbColorSize
		true,							// useStencilSize
		1,								// stencilSize
		false,							// useRendererID
		AGL_RENDERER_GENERIC_ID,		// rendererID
		false,							// permitAnyRenderer
		false,							// useNoRecovery
		false							// useAccelerated
		};
	GLint attrib[ kOGLMaxAttribArraySize ];
	
		// note that we (probably) should never have
		// BOTH gOpenGLForceGenericRenderer & gOpenGLPermitAnyRenderer
	if ( gOpenGLForceGenericRenderer )
		{
		requested.useRendererID = true;				// force the software renderer for testing
//		requested.rendererID = AGL_RENDERER_GENERIC_ID;
//		requested.useAccelerated = false;
		}
	else
		{
		requested.useAccelerated = true;			// only consider accelerated renderers
		
		// this allows non-compliant hardware (Rage II) to be used
		if ( gOpenGLPermitAnyRenderer )
			requested.permitAnyRenderer = true;
		}
	
	populatePixelAttributeArray( requested, attrib );
	AGLPixelFormat fmt = aglChoosePixelFormat( nullptr, 0, attrib );
	
	if ( not fmt )
		{
		ShowMessage( "aglChoosePixelFormat() returned NULL, "
			"retrying with useStencilSize = false..." );
		requested.useStencilSize = false;
		populatePixelAttributeArray( requested, attrib );
		fmt = aglChoosePixelFormat( nullptr, 0, attrib );
		}
	
	if ( not fmt && requested.useAccelerated )
		{
		ShowMessage( "aglChoosePixelFormat() returned NULL, "
			"retrying with useAccelerated = false, useStencilSize = true..." );
		requested.useAccelerated = false;
		requested.useStencilSize = true;
		populatePixelAttributeArray( requested, attrib );
		fmt = aglChoosePixelFormat( nullptr, 0, attrib );
		}
	
		// the software renderer has a stencil buffer, so we aren't going to consider
		// useAccelerated = false, useStencilSize = false
	
	if ( not fmt )
		ShowMessage( "aglChoosePixelFormat() returned NULL" );
	else
		{
		ctx = aglCreateContext( fmt, nullptr );
	
		if ( not ctx
		||   SetOGLDrawable() == GL_FALSE )
			{
			if ( not ctx )
				ShowMessage( "aglCreateContext() failed" );
			else
				{
				ShowMessage( "SetOGLDrawable(ctx) failed" );
				aglDestroyContext( ctx );	// ignore errors
				ctx = nullptr;
				}
			}
		else
			{
			if ( aglSetCurrentContext( ctx ) == GL_FALSE )
				{
				aglDestroyContext( ctx );
				ctx = nullptr;
				ShowMessage( "aglSetCurrentContext() failed" );
				}
			else
				{
				gOGLRendererString = reinterpret_cast<const char *>( glGetString( GL_RENDERER ) );
				gOGLRendererRage128 = strstr( gOGLRendererString, "Rage 128" );
				gOGLRendererGeneric = strstr( gOGLRendererString, "Generic" );
				
				if ( gOGLRendererRage128 /*&& gOGL_UsingOSXVersion*/ )	// Rage 128 under OSX
					{
					if ( gOpenGLPermitAnyRenderer )	// force-enable
						ShowMessage( "Using Rage 128 renderer under OSX; a crash is likely..." );
					else
						{
						ShowMessage( "Found (and rejected) Rage 128 renderer..." );
						if ( not aglDestroyContext( ctx ) )
							ShowAGLErrors();
						ctx = nullptr;
						aglDestroyPixelFormat( fmt );
						ShowAGLErrors();
						fmt = nullptr;
						
						requested.useRendererID = true;	// force the software renderer
//						requested.rendererID = AGL_RENDERER_GENERIC_ID;
						requested.useAccelerated = false;
						}
					}
				
				if ( ctx )
					{
						// Rage II should try to match screen depth
						// Rage Pro should try to match screen depth
					if ( strstr( gOGLRendererString, "Rage II" )
					||   strstr( gOGLRendererString, "Rage Pro" ) )
						{
						DShowMessage( "unsetting depth size..." );
						if ( not aglDestroyContext( ctx ) )
							ShowAGLErrors();
						ctx = nullptr;
						aglDestroyPixelFormat( fmt );
						ShowAGLErrors();
						fmt = nullptr;
						
							// remove the color depth attribs
						requested.useRGBColorSize = false;
						}
					}
				
				if ( not ctx )
					{
						// reset the stencil flag, if we had changed it
					requested.useStencilSize = true;
					populatePixelAttributeArray( requested, attrib );
					fmt = aglChoosePixelFormat( nullptr, 0, attrib );
					
					if ( not fmt )
						{
						ShowMessage( "aglChoosePixelFormat() returned NULL, "
							"retrying with useStencilSize = false..." );
						requested.useStencilSize = false;
						populatePixelAttributeArray( requested, attrib );
						fmt = aglChoosePixelFormat( nullptr, 0, attrib );
						}
					
					if ( not fmt && requested.useAccelerated )
						{
						ShowMessage( "aglChoosePixelFormat() returned NULL, "
							"retrying with useAccelerated = false, useStencilSize = true..." );
						requested.useAccelerated = false;
						requested.useStencilSize = true;
						populatePixelAttributeArray( requested, attrib );
						fmt = aglChoosePixelFormat( nullptr, 0, attrib );
						}
					
					if ( not fmt )
						ShowMessage( "aglChoosePixelFormat() returned NULL" );
					else
						{
						ctx = aglCreateContext( fmt, nullptr );
						if ( SetOGLDrawable() == GL_FALSE )
							{
							ShowMessage( "SetOGLDrawable() failed" );
							aglDestroyContext( ctx );	// ignore errors
							ctx = nullptr;
							}
						else
							{
							if ( aglSetCurrentContext( ctx ) == GL_FALSE )
								{
								aglDestroyContext( ctx );
								ctx = nullptr;
								ShowMessage( "aglSetCurrentContext() failed" );
								}
							else
								{
								gOGLRendererString = reinterpret_cast<const char *>(
									glGetString( GL_RENDERER ) );
								gOGLRendererRage128 =
									strstr( gOGLRendererString, "Rage 128" );
								gOGLRendererGeneric =
									strstr( gOGLRendererString, "Generic" );
								}
							}
						}
					}
				
				gOGLManageTexturePriority = false;
					// was: ( gOGLRendererRage128 && not gOGL_UsingOSXVersion );
				}
			}
		}
	
	if ( not ctx )
		ShowMessage( "attachOGLContextToGameWindow() failed" );
	else
		{
		if ( gOGLRendererRage128
		||   strstr( gOGLRendererString, "Rage Pro" )
		||   strstr( gOGLRendererString, "Rage II" ) )
			{
			gOGLRendererSmallTextureMemory = true;
			}
		else
			gOGLRendererSmallTextureMemory = false;
		
		if ( strstr( gOGLRendererString, "Rage Pro" )
		||   strstr( gOGLRendererString, "Rage II" ) )
			{
			gOGLDisableShadowEffects = true;
			}
		else
			gOGLDisableShadowEffects = false;
			
#ifdef BROKEN_TEXSUBIMAGE2D_COLOR_INDEX
		if (
			( gOSFeatures.osVersionMajor >= 10 && gOSFeatures.osVersionMinor >= 5 )	// 10.5 bug
				&&
			(
					// what i really want is to match the driver version
					// (the other half of GL_VERSION) against the renderer ID
					// but I better doublecheck that on the mailing list first
				strstr( gOGLRendererString, "NVIDIA GeForce4 MX OpenGL Engine" )
					||
				strstr( gOGLRendererString, "NVIDIA GeForce 6600 OpenGL Engine" )
// || 1	// for testing
			)
		) gOGLBrokenTexSubImage_ColorIndex = true;
#endif 	// BROKEN_TEXSUBIMAGE2D_COLOR_INDEX
		
		if ( not aglDescribePixelFormat( fmt, AGL_RENDERER_ID, &gOGLRendererID ) )
			gOGLRendererID = 0;
		
#if defined( DEBUG_VERSION ) && 0
		DShowMessage( "\traw AGL_RENDERER_ID == 0x%x", gOGLRendererID );
#endif	// DEBUG_VERSION
		
		gOGLRendererID &= 0x00FFFF00;	// per (apple's) john stauffer on the mac-opengl list
		
#if defined( DEBUG_VERSION ) && 0
		switch ( gOGLRendererID )
			{
			case AGL_RENDERER_GENERIC_ID:
				DShowMessage( "\tAGL_RENDERER_ID == 0x%x (AGL_RENDERER_GENERIC_ID)",
					(uint) gOGLRendererID );
				break;
			
			case AGL_RENDERER_APPLE_SW_ID:
				DShowMessage( "\tAGL_RENDERER_ID == 0x%x (AGL_RENDERER_APPLE_SW_ID)",
					(uint) gOGLRendererID );
				break;

			case AGL_RENDERER_ATI_RAGE_128_ID:
				DShowMessage( "\tAGL_RENDERER_ID == 0x%x (AGL_RENDERER_ATI_RAGE_128_ID)",
					(uint) gOGLRendererID );
				break;

			case AGL_RENDERER_ATI_RADEON_ID:
				DShowMessage( "\tAGL_RENDERER_ID == 0x%x (AGL_RENDERER_ATI_RADEON_ID)",
					(uint) gOGLRendererID );
				break;

			case AGL_RENDERER_ATI_RAGE_PRO_ID:
				DShowMessage( "\tAGL_RENDERER_ID == 0x%x (AGL_RENDERER_ATI_RAGE_PRO_ID)",
					(uint) gOGLRendererID );
				break;

			case AGL_RENDERER_ATI_RADEON_8500_ID:		// also 9000
				DShowMessage( "\tAGL_RENDERER_ID == 0x%x (AGL_RENDERER_ATI_RADEON_8500_ID)",
					(uint) gOGLRendererID );
				break;

			case AGL_RENDERER_ATI_RADEON_9700_ID:
				DShowMessage( "\tAGL_RENDERER_ID == 0x%x (AGL_RENDERER_ATI_RADEON_9700_ID)",
					(uint) gOGLRendererID );
				break;

			case AGL_RENDERER_NVIDIA_GEFORCE_2MX_ID:	// also 4MX
				DShowMessage(
					"\tAGL_RENDERER_ID == 0x%x (AGL_RENDERER_NVIDIA_GEFORCE_2MX_ID, 4MX)",
					(uint) gOGLRendererID );
				break;

			case AGL_RENDERER_NVIDIA_GEFORCE_3_ID:		// also 4Ti
				DShowMessage( "\tAGL_RENDERER_ID == 0x%x (AGL_RENDERER_NVIDIA_GEFORCE_3_ID, 4Ti)",
					(uint) gOGLRendererID );
				break;

			case AGL_RENDERER_NVIDIA_GEFORCE_FX_ID:
				DShowMessage( "\tAGL_RENDERER_ID == 0x%x (AGL_RENDERER_NVIDIA_GEFORCE_FX_ID)",
					(uint) gOGLRendererID );
				break;

			case AGL_RENDERER_MESA_3DFX_ID:
				DShowMessage( "\tAGL_RENDERER_ID == 0x%x (AGL_RENDERER_MESA_3DFX_ID)",
					(uint) gOGLRendererID );
				break;

			default:
				DShowMessage( "\tAGL_RENDERER_ID == 0x%x (unknown)", (uint) gOGLRendererID );
				break;
			}
#endif	// DEBUG_VERSION
		
		gOGLBlendingIsEnabled = false;
		gOGLAlphaTestIsEnabled = false;
		gOGLCurrentEnabledTextureTarget = 0;
		gOGLTextureEnvironmentMode = -1;	// not a valid value
		gOGLCurrentTexture = 0;
		gOGLBlendEquationMode = GL_FUNC_ADD;
		gOGLPixelZoomIsEnabled = false;
		gOGLSelectionList = 0;	// the old displaylist was implicitly destroyed
								// when the old context was destroyed
		gOGLNightTexture = 0;
		gOGLNightTexturePriority = -1.0f;	// not a valid value
		gLightList = 0;
		
#ifdef OGL_USE_VERTEX_ARRAYS
		gOGLVertexArray = nullptr;
		gOGLTexCoordArray = nullptr;
#endif	// OGL_USE_VERTEX_ARRAYS
		
		GLint rvalue, gvalue, bvalue;
		if ( not aglDescribePixelFormat( fmt, AGL_RED_SIZE, &rvalue ) )		rvalue = 0;
		if ( not aglDescribePixelFormat( fmt, AGL_GREEN_SIZE, &gvalue ) )	gvalue = 0;
		if ( not aglDescribePixelFormat( fmt, AGL_BLUE_SIZE, &bvalue ) )	bvalue = 0;
		
		if ( rvalue >= 8 && gvalue >= 8 && bvalue >= 8 && not gOGLRendererGeneric )
			gOGLUseDither = false;
		else
			gOGLUseDither = true;
		
		GLint stencilvalue;
		if ( not aglDescribePixelFormat( fmt, AGL_STENCIL_SIZE, &stencilvalue ) )
			stencilvalue = 0;
		gOGLStencilBufferAvailable = (stencilvalue > 0);
		
#if defined(DEBUG_VERSION) && 1
		GLint avalue;
		if ( not aglDescribePixelFormat( fmt, AGL_ALPHA_SIZE, &avalue ) )	avalue = 0;
		DShowMessage( "\tRGBA size == %d %d %d %d, stencil size == %d",
			(int) rvalue, (int) gvalue, (int) bvalue, (int) avalue, (int) stencilvalue );
		
//		GLint auxvalue;
//		if ( not aglDescribePixelFormat (fmt, AGL_AUX_BUFFERS, &auxvalue ) ) auxvalue = 0;
//		DShowMessage( "\tAGL_AUX_BUFFERS == %d", auxvalue );
		
//		GLint depthvalue;
//		if ( not aglDescribePixelFormat (fmt, AGL_DEPTH_SIZE, &depthvalue ) ) depthvalue = 0;
//		DShowMessage( "\tAGL_DEPTH_SIZE == %d", depthvalue );
		
# ifdef OGL_USE_AGL_STATE_VALIDATION
		if ( aglEnable( ctx, AGL_STATE_VALIDATION ) )
			DShowMessage( "AGL_STATE_VALIDATION is enabled for context" );
		else
			{
			ShowMessage( "AGL_STATE_VALIDATION could not be enabled for context" );
			ShowAGLErrors();
			}
# endif  // OGL_USE_AGL_STATE_VALIDATION
		
# if 0
		DShowMessage( "\tGL_VENDOR == %s",		glGetString(GL_VENDOR) );
		DShowMessage( "\tGL_RENDERER == %s",	glGetString(GL_RENDERER) );
		DShowMessage( "\tGL_VERSION == %s",		glGetString(GL_VERSION) );
		DShowMessage( "\tGL_EXTENSIONS == %s",	glGetString(GL_EXTENSIONS) );
		DShowMessage( "\tGLU_VERSION == %s",	gluGetString(GLU_VERSION) );
		DShowMessage( "\tGLU_EXTENSIONS == %s",	gluGetString(GLU_EXTENSIONS) );
# endif  // 1
		
		GLint maxtexsize;
		glGetIntegerv( GL_MAX_TEXTURE_SIZE, &maxtexsize );
		DShowMessage( "\t(theoretical) GL_MAX_TEXTURE_SIZE == %d", (int) maxtexsize );
		
#endif	// DEBUG_VERSION
		
		const GLubyte * p = glGetString( GL_VERSION );
			// SUPPOSED to be in the form #.#<space><text> or #.#.#<space><text>
			// where # can be one or more digits
			
			//  unfortunately, MacOS 9 GL is broken
		
		int majorVersion = 0;
		while ( isdigit(*p) )	// - because os9's gl is broken
			{
			majorVersion *= 10;
			majorVersion += *p - '0';
			++p;
			}
		
		int minorVersion = 0;
		++p;	// skip delimiter
		while ( isdigit(*p) )
			{
			minorVersion *= 10;
			minorVersion += *p - '0';
			++p;
			}
		
		int releaseVersion = 0;
		if ( '.' == *p )	// only happens if release version !=0
			{
			++p;	// skip delimiter
			while ( isdigit(*p) )
				{
				releaseVersion *= 10;
				releaseVersion += *p - '0';
				++p;
				}
			}
		
		if ( majorVersion >= 2
		||   ( majorVersion == 1 && minorVersion >= 2 ) )
			gOGLHaveVersion_1_2 = GL_TRUE;
		else
			gOGLHaveVersion_1_2 = GL_FALSE;
		
		if ( majorVersion >= 2
		||   ( majorVersion == 1 && minorVersion >= 3 ) )
			gOGLHaveVersion_1_3 = GL_TRUE;
		else
			gOGLHaveVersion_1_3 = GL_FALSE;
		
		if ( majorVersion >= 2
		||   ( majorVersion == 1 && minorVersion >= 4 ) )
			gOGLHaveVersion_1_4 = GL_TRUE;
		else
			gOGLHaveVersion_1_4 = GL_FALSE;
		
#ifdef OGL_USE_TEXTURE_RECTANGLE
		gOGLHaveTexRectExtension = HaveExtension( "GL_EXT_texture_rectangle" )
			|| HaveExtension( "GL_NV_texture_rectangle" );
#else
		gOGLHaveTexRectExtension = false;	// for testing that fricking bug....
#endif	// OGL_USE_TEXTURE_RECTANGLE
		
		if ( gOGLHaveTexRectExtension )
			{
			gOGLTexTarget2D = GL_TEXTURE_RECTANGLE_EXT;
			gOGLProxyTexTarget2D = GL_PROXY_TEXTURE_RECTANGLE_EXT;
			}
		else
			{
			gOGLTexTarget2D = GL_TEXTURE_2D;
			gOGLProxyTexTarget2D = GL_PROXY_TEXTURE_2D;
			}
		
#ifdef OGL_DETECT_PALETTED_TEXTURE
		
		gOGLHavePalettedTextureExtension = HaveExtension( "GL_EXT_paletted_texture" );
		
# if 1	// Apple DTS requested a version with this removed.
		if (
			(
/*
				( gOSFeatures.osVersion == 0x01034 )
					||
				( gOSFeatures.osVersion == 0x01035 )	// odds that it will be fixed in 10.3.6?
*/
				   (gOSFeatures.osVersionMajor == 10)
				&& (gOSFeatures.osVersionMinor ==  3)
				&& (gOSFeatures.osVersionBugFix == 4 || gOSFeatures.osVersionBugFix == 5)
			)
				&&
			(
				( gOGLRendererID == AGL_RENDERER_NVIDIA_GEFORCE_2MX_ID )	// also 4MX
					||
				( gOGLRendererID == AGL_RENDERER_NVIDIA_GEFORCE_3_ID )		// also 4Ti
					||
				( gOGLRendererID == AGL_RENDERER_NVIDIA_GEFORCE_FX_ID )
			)
		)
			{
			gOGLHavePalettedTextureExtension = true;
			}
# endif  // 1
		
#else	// ! OGL_DETECT_PALETTED_TEXTURE
		
			// odds that it will EVER be fixed?
		
		gOGLHavePalettedTextureExtension = true;
			// because we don't want to use it, we're only interested in
			// its effect on the definition of GL_EXT_texture_rectangle
		
#endif	// OGL_DETECT_PALETTED_TEXTURE
		
		gOGLHaveBlendSubtractCoreFunction = gOGLHaveVersion_1_4
			|| HaveExtension( "GL_ARB_imaging" );
		gOGLHaveBlendSubtractExtension = HaveExtension( "GL_EXT_blend_subtract" );
		gOGLHaveBlendSubtract = gOGLHaveBlendSubtractCoreFunction
			|| gOGLHaveBlendSubtractExtension;
		
		if ( gOGLHaveBlendSubtract )
			{
			if ( gOGLHaveBlendSubtractCoreFunction )
				gOGLglBlendEquation = &glBlendEquation;
			else
				gOGLglBlendEquation = &glBlendEquationEXT;
				
			// the docs say i have to test the function pointer, just in case
			if ( not gOGLglBlendEquation )
				gOGLHaveBlendSubtract = false;
			}
		
#if 0 // we cannot run on 10.3.x anymore
	
	// i'm still not sure this is a problem,
	// but I don't have access to a system on
	// which to confirm or deny the bug report
		if (
			(
//				gOSFeatures.osVersion == 0x01034
				gOSFeatures.osVersionMajor == 10
				&& gOSFeatures.osVersionMinor == 3
				&& ( gOSFeatures.osVersionBugFix == 4 || gOSFeatures.osVersionBugFix == 5 )
			)
				&&
			( gOGLRendererID == AGL_RENDERER_ATI_RADEON_8500_ID )	// also 9000
		)
			{
			gOGLHaveBlendSubtract = false;
			}
#endif  // 0
		
		gOGLHaveClampToEdge = gOGLHaveVersion_1_2 || HaveExtension( "GL_SGIS_texture_edge_clamp" );
		gOGLHaveBGRA = gOGLHaveVersion_1_2 || HaveExtension( "GL_EXT_bgra" );
		gOGLHavePackedPixels = gOGLHaveVersion_1_2 || HaveExtension( "GL_APPLE_packed_pixels" );
		
#ifdef DEBUG_VERSION
		DShowMessage( "numeric GL version: <%d.%d.%d>",
			majorVersion, minorVersion, releaseVersion );
		
#define Emit1Bool(x) DShowMessage( "%s = %s", #x, x ? "TRUE" : "FALSE" );
		
		Emit1Bool( gOGLHaveVersion_1_2 );
		Emit1Bool( gOGLHaveVersion_1_3 );
		Emit1Bool( gOGLHaveVersion_1_4 );
		
		if ( gOGLHaveTexRectExtension )
			{
			DShowMessage( "GL_EXT_texture_rectangle is available" );
			glGetIntegerv( GL_MAX_RECTANGLE_TEXTURE_SIZE_EXT, &maxtexsize );
			DShowMessage( "\t(theoretical) GL_MAX_RECTANGLE_TEXTURE_SIZE_EXT == %d",
				(int) maxtexsize );
			}
		else
			{
			DShowMessage( "GL_EXT_texture_rectangle is NOT available" );
			}
		
		if ( gOGLHavePalettedTextureExtension )
			{
			DShowMessage( "GL_EXT_paletted_texture is available" );
			}
		else
			{
			DShowMessage( "GL_EXT_paletted_texture is NOT available" );
			}
		
		Emit1Bool( gOGLHaveBlendSubtract );
		Emit1Bool( gOGLHaveClampToEdge );
		Emit1Bool( gOGLHaveBGRA );
		Emit1Bool( gOGLHavePackedPixels );
		Emit1Bool( gOGLBrokenTexSubImage_ColorIndex );
		
#endif	// DEBUG_VERSION
		
		glGetIntegerv( GL_MAX_PIXEL_MAP_TABLE, &gPixelMapSize );
		if ( gPixelMapSize > 256 )
			gPixelMapSize = 256;
		
			// not likely we will ever get < 256, but if we do we're screwed
//		IndexToAlphaMap[kIndexToAlphaMapAlpha100][0] = 0xffff;
		IndexToAlphaMap[kIndexToAlphaMapAlpha100TransparentZero][0] = 0x0000;
		IndexToAlphaMap[kIndexToAlphaMapAlpha25][0] = 0xffff * 0.25f;
		IndexToAlphaMap[kIndexToAlphaMapAlpha25TransparentZero][0] = 0x0000;
		IndexToAlphaMap[kIndexToAlphaMapAlpha50][0] = 0xffff * 0.5f;
		IndexToAlphaMap[kIndexToAlphaMapAlpha50TransparentZero][0] = 0x0000;
		IndexToAlphaMap[kIndexToAlphaMapAlpha75][0] = 0xffff * 0.75f;
		IndexToAlphaMap[kIndexToAlphaMapAlpha75TransparentZero][0] = 0x0000;
		
		for ( int i = 1; i < gPixelMapSize; ++i )
			{
//			IndexToAlphaMap[kIndexToAlphaMapAlpha100][i] = 0xffff;
			IndexToAlphaMap[kIndexToAlphaMapAlpha100TransparentZero][i] = 0xffff;
			IndexToAlphaMap[kIndexToAlphaMapAlpha25][i] = 0xffff * 0.25f;
			IndexToAlphaMap[kIndexToAlphaMapAlpha25TransparentZero][i] = 0xffff * 0.25f;
			IndexToAlphaMap[kIndexToAlphaMapAlpha50][i] = 0xffff * 0.5f;
			IndexToAlphaMap[kIndexToAlphaMapAlpha50TransparentZero][i] = 0xffff * 0.5f;
			IndexToAlphaMap[kIndexToAlphaMapAlpha75][i] = 0xffff * 0.75f;
			IndexToAlphaMap[kIndexToAlphaMapAlpha75TransparentZero][i] = 0xffff * 0.75f;
			}
#if 0
		gIndexToRedMap = NEW_TAG("gIndexToRedMap") GLushort[gPixelMapSize];
		gIndexToGreenMap = NEW_TAG("gIndexToGreenMap") GLushort[gPixelMapSize];
		gIndexToBlueMap = NEW_TAG("gIndexToBlueMap") GLushort[gPixelMapSize];
		initOGLIndexToColorTables( gIndexToRedMap, gIndexToGreenMap, gIndexToBlueMap,
			gPixelMapSize );
#else
		if ( not gIndexToRedMap )
			{
			GetAppData( 'BTbl', 5, reinterpret_cast< void** >( &gIndexToRedMap ), nullptr );
			GetAppData( 'BTbl', 6, reinterpret_cast< void** >( &gIndexToGreenMap ), nullptr );
			GetAppData( 'BTbl', 7, reinterpret_cast< void** >( &gIndexToBlueMap ), nullptr );
			}
#endif  // 0
		
		glPixelMapusv( GL_PIXEL_MAP_I_TO_R, gPixelMapSize, gIndexToRedMap );
		glPixelMapusv( GL_PIXEL_MAP_I_TO_G, gPixelMapSize, gIndexToGreenMap );
		glPixelMapusv( GL_PIXEL_MAP_I_TO_B, gPixelMapSize, gIndexToBlueMap );
//		glPixelMapusv( GL_PIXEL_MAP_I_TO_A, gPixelMapSize, 
//			IndexToAlphaMap[ kIndexToAlphaMapAlpha100TransparentZero ] );
			// kIndexToAlphaMapCount isn't a valid id, forcing a mismatch on the first compare
		gOGLCurrentIndexToAlphaMap = kIndexToAlphaMapCount;
		}
	
	if ( fmt )
		aglDestroyPixelFormat( fmt );
	
	if ( not aglConfigure( AGL_CLEAR_FORMAT_CACHE, 1 ) )	// minimize memory
		{
		ShowMessage( "aglConfigure(AGL_CLEAR_FORMAT_CACHE) returned error:" );
		ShowOpenGLErrors();
		}
	
	ShowOpenGLErrors();
}


/*
**	setupOpenGLGameWindow()
*/
void
setupOpenGLGameWindow( AGLContext targetContext, bool visible )
{
	AGLContext currentContext = aglGetCurrentContext();
	aglSetCurrentContext( targetContext );
	
	GLint params[4];
	if ( not visible )
		{
		params[0] = -1;	// x; i think this is right to move it outside the window's bounds...
		params[1] = -1;	// y
		params[2] = 1;	// width; can't use width or height of 0
		params[3] = 1;	// height
		}
	else
		{
			// these are in GL window coordinates (0 at bottomleft, positive upright)
		params[0] = gLayout.layoFieldBox.rectLeft;
		params[1] = gLayout.layoWinHeight - gLayout.layoFieldBox.rectBottom;
			// dist from bottom of window to bottom of field
		params[2] = gLayout.layoFieldBox.rectRight - gLayout.layoFieldBox.rectLeft;
		params[3] = gLayout.layoFieldBox.rectBottom - gLayout.layoFieldBox.rectTop;
		
#if defined(DEBUG_VERSION) && 0
		DShowMessage( "gLayout.layoWinWidth, gLayout.layoWinHeight: %d %d",
				gLayout.layoWinWidth, gLayout.layoWinHeight );
		DShowMessage( "gLayout.layoFieldBox (x y w h): %d %d %d %d",
				gLayout.layoFieldBox.rectLeft,
				gLayout.layoFieldBox.rectTop,
				gLayout.layoFieldBox.rectRight - gLayout.layoFieldBox.rectLeft,
				gLayout.layoFieldBox.rectBottom - gLayout.layoFieldBox.rectTop );
		DShowMessage( "gLayout.layoFieldBox (l t r b): %d %d %d %d",
				gLayout.layoFieldBox.rectLeft,
				gLayout.layoFieldBox.rectTop,
				gLayout.layoFieldBox.rectRight,
				gLayout.layoFieldBox.rectBottom );
#endif	// DEBUG_VERSION
		
		glMatrixMode( GL_PROJECTION );
		glLoadIdentity();
//		glScalef( 1.0f, -1.0f, 1.0f );	// cuz cl is cast in mac os screen coordinates
		
#if RESIZING_EXPERIMENT
		// are we scaled?
		int minDimen = params[2];
		if ( params[3] < minDimen )
			minDimen = params[3];
		gScaleFactor = 540.0F / float(minDimen);	// standard large window width
		
		int cx = params[0] + params[2] / 2;
		int cy = params[1] + params[3] / 2;
		float halfWidth = gScaleFactor * params[2] / 2;
		float halfHeight = gScaleFactor * params[3] / 2;
#endif  // RESIZING_EXPERIMENT
		
#if 1
		glViewport( 0, 0, params[2], params[3] );
			// note how the sense of Top and Bottom are reversed here...
//		gluOrtho2D( gLayout.layoFieldBox.rectLeft, gLayout.layoFieldBox.rectRight,
//					gLayout.layoFieldBox.rectTop, gLayout.layoFieldBox.rectBottom );
# if RESIZING_EXPERIMENT
		gluOrtho2D( cx - halfWidth, cx + halfWidth,
					cy + halfHeight, cy - halfHeight );
# else
		gluOrtho2D( gLayout.layoFieldBox.rectLeft, gLayout.layoFieldBox.rectRight,
					gLayout.layoFieldBox.rectBottom, gLayout.layoFieldBox.rectTop );
# endif  // RESIZING_EXPERIMENT
#else		// never finished debugging this...
			// for windows version, better to use glAddSwapHintRectWIN (use wglGetProcAddress)
			// but having a 'pure' OGL mechanism is good too, perhaps for a linux version...
		glViewport( 0, 0, gLayout.layoWinWidth, gLayout.layoWinHeight );
		gluOrtho2D( 0, gLayout.layoWinWidth, 0, gLayout.layoWinHeight );
		glScissor( params[0], params[1], params[2], params[3] );
		glEnable( GL_SCISSOR_TEST );
#endif  // 1
		glMatrixMode( GL_MODELVIEW );
		glLoadIdentity();
		
			// set up some things that we use everywhere
		
#if 1	// might not keep this, it didn't seem to help much
		// to do: needs feature test, etc etc
		// to do: should i make this depend on gFastBlendMode instead of gOGLRendererGeneric?
		// (it will have to be moved elsewhere if so)
		glHint( GL_TRANSFORM_HINT_APPLE, gOGLRendererGeneric ? GL_FASTEST : GL_DONT_CARE );
#endif
		glBlendFunc( GL_SRC_ALPHA, GL_ONE_MINUS_SRC_ALPHA );
		glAlphaFunc( GL_NOTEQUAL, 0.0f );
		glDisable( GL_DITHER );
			// this means "we are only interested in the lowest bit;
			// if it's not 1 we PASS, otherwise FAIL"
		glStencilFunc( GL_NOTEQUAL, 0x1, 0x1 );
		
			// this means "if we fail the test, don't change the stencil buffer;
			// if we pass, replace it with 1"
		glStencilOp( GL_KEEP, GL_REPLACE, GL_REPLACE );
		
#ifdef OGL_USE_VERTEX_ARRAYS
		glEnableClientState( GL_VERTEX_ARRAY );
		glEnableClientState( GL_TEXTURE_COORD_ARRAY );
#endif	// OGL_USE_VERTEX_ARRAYS
		}
	
	if ( aglSetInteger( targetContext, AGL_BUFFER_RECT, params ) )
		{
		if ( not aglEnable( targetContext, AGL_BUFFER_RECT ) )
			{
			ShowMessage( "aglEnable(AGL_BUFFER_RECT) failed" );
			ShowAGLErrors();
#if defined(DEBUG_VERSION) && 0
			}
		else
			{
			DShowMessage( "using buffer rect (x,y,w,h) %d %d %d %d",
				params[0], params[1], params[2], params[3] );
#endif	// DEBUG_VERSION
			}
		}
	else
		{
		ShowMessage( "aglSetInteger(AGL_BUFFER_RECT) failed" );
		ShowAGLErrors();
		}
	
#ifdef OGL_USE_AGL_SWAP_INTERVAL
	const GLint AGLSwapInterval = 1;
	if ( aglSetInteger( targetContext, AGL_SWAP_INTERVAL, &AGLSwapInterval ) )
		DShowMessage( "\tenabled AGL_SWAP_INTERVAL" );
	else
		{
		ShowMessage( "aglSetInteger(AGL_SWAP_INTERVAL) failed" );
		ShowAGLErrors();
		}
#endif	// OGL_USE_AGL_SWAP_INTERVAL
	
	aglUpdateContext( targetContext );
		// aglUpdateContext must be called for any move, zoom, resize
		// it also needs to FOLLOW mucking with AGL_BUFFER_RECT
	
	aglSetCurrentContext( currentContext );
	
	ShowOpenGLErrors();
}


/*
**	drawOGLText()
**	shorthand for the below function
*/
void
drawOGLText( GLint x, GLint y, GLuint fontListBase, const char * text )
{
	if ( text && *text )
		drawOGLText( x, y, fontListBase, text, strlen(text) );
}


/*
**	drawOGLText()
**	draw some text at a given location
*/
void
drawOGLText( GLint x, GLint y, GLuint fontListBase, const char * text, GLsizei length )
{
	if ( not text || 0 == length )
		return;
	
#if RESIZING_EXPERIMENT
	GLfloat tx = (x - gFieldCenter.ptH) / gScaleFactor;
	GLfloat ty = (y - gFieldCenter.ptV) / gScaleFactor;
# define HalfPix		(0.0F)		// (0.5F)
#else
	GLfloat tx = (x - gFieldCenter.ptH);
	GLfloat ty = (y - gFieldCenter.ptV);
# define HalfPix		(0.5F)
#endif  // RESIZING_EXPERIMENT
	
	glListBase( fontListBase );
		glRasterPos2i( gFieldCenter.ptH, gFieldCenter.ptV );	// guaranteed valid
		glBitmap( 0, 0, 0, 0, tx + HalfPix, -ty - HalfPix, nullptr );
				// -v cuz of glScale(1, -1)
		glCallLists( length, GL_UNSIGNED_BYTE, text );
	glListBase( 0 );	// reset to the default value
}


/*
**	drawOGLPixmap()
**	blit a pixmap, the (more or less) straightforward way
*/
void
drawOGLPixmap(	const DTSRect& src,
				const DTSRect& dst,
				GLuint rowLength,
				IndexToAlphaMapID alphamap,
				void * pixels )
{
	disableTexturing();
	glRasterPos2i( gFieldCenter.ptH, gFieldCenter.ptV );	// guaranteed valid
	glBitmap(	0, 0, 0, 0,
				dst.rectLeft - gFieldCenter.ptH,
				-(dst.rectBottom - gFieldCenter.ptV + src.rectTop - src.rectBottom),
				nullptr );	// -v cuz of glScale(1, -1)
	if ( kIndexToAlphaMapNone == alphamap )
		{
		disableBlending();
		disableAlphaTest();
		}
	else
		{
		enableBlending();
		enableAlphaTest();
		setIndexToAlphaMap( alphamap );
		}
	
	if ( src.rectTop )
		glPixelStorei( GL_UNPACK_SKIP_ROWS, src.rectTop );
	
	if ( src.rectLeft )
		glPixelStorei( GL_UNPACK_SKIP_PIXELS, src.rectLeft );
	
	if ( src.rectRight - src.rectLeft != (int) rowLength )
		glPixelStorei( GL_UNPACK_ROW_LENGTH, rowLength );
	
		// the pixelzoom state needs to be moved into a global, to avoid
		// all these unnecessary state changes
	if ( not gOGLPixelZoomIsEnabled )
		glPixelZoom( 1.0f, -1.0f );
	
	glDrawPixels(	src.rectRight - src.rectLeft,
					src.rectBottom - src.rectTop,
					GL_COLOR_INDEX, GL_UNSIGNED_BYTE,
					pixels );
	if ( not gOGLPixelZoomIsEnabled )
		glPixelZoom( 1.0f, 1.0f );
	
	if ( src.rectTop )
		glPixelStorei( GL_UNPACK_SKIP_ROWS, 0 );
	
	if ( src.rectLeft )
		glPixelStorei( GL_UNPACK_SKIP_PIXELS, 0 );
	
	if ( src.rectRight - src.rectLeft != (int) rowLength )
		glPixelStorei( GL_UNPACK_ROW_LENGTH, 0 );
	
#if 0 && defined( DEBUG_VERSION )
	glColor3us( 0xffff, 0x0000, 0x0000 );		// red, cuz drawing this way is slow
	glBegin( GL_LINE_LOOP );
		glVertex2f( dst.rectLeft + 0.5f, dst.rectTop + 0.5f );
		glVertex2f( dst.rectRight - 0.5f, dst.rectTop + 0.5f );
		glVertex2f( dst.rectRight - 0.5f, dst.rectBottom - 0.5f );
		glVertex2f( dst.rectLeft + 0.5f, dst.rectBottom - 0.5f );
	glEnd();
	glBegin( GL_LINES );
		glVertex2f( dst.rectLeft + 0.5f, dst.rectTop + 0.5f );
		glVertex2f( dst.rectRight - 0.5f, dst.rectBottom - 0.5f );
		glVertex2f( dst.rectRight - 0.5f, dst.rectTop + 0.5f );
		glVertex2f( dst.rectLeft + 0.5f, dst.rectBottom - 0.5f );
	glEnd();
#endif	// DEBUG_VERSION
}


/*
**	drawOGLPixmapAsTexture()
**	blit a pixmap, as a texture
*/
void
drawOGLPixmapAsTexture(
	const DTSRect& src,
	const DTSRect& dst,
	GLuint rowLength,
	IndexToAlphaMapID alphamap,
	TextureObject::TextureObjectType textureObjectType,
	ImageCache * cache,
	GLfloat targetAlpha	/* = 1.0f */ )
{
#ifdef DEBUG_VERSION
	ImageCache * testcache = dynamic_cast<ImageCache *>( cache );
	if ( not testcache )
		{
		ShowMessage( "fake ImageCache pointer passed to drawOGLPixmapAsTexture()" );
		drawOGLPixmap( src, dst, rowLength, alphamap, cache->icImage.cliImage.GetBits() );
		return;
		}
#endif	// DEBUG_VERSION
	
	if ( not cache->textureObject )
		{
		switch ( textureObjectType )
			{
			case TextureObject::kNoTexture:
				ShowMessage( "drawOGLPixmapAsTexture(): kNoTexture encountered" );
				break;
			
			case TextureObject::kSingleTile:
				cache->textureObject = NEW_TAG("TexObjSingle") TextureObjectSingle;
				break;
			
			case TextureObject::kNAnimatedTiles:
				cache->textureObject = NEW_TAG("TexObjNAnimated") TextureObjectNAnimated;
				break;
			
			case TextureObject::kMobileArray:
				cache->textureObject = NEW_TAG("TexObjMobileArray") TextureObjectMobileArray;
				break;
			
			case TextureObject::kBalloonArray:
				cache->textureObject = NEW_TAG("TexObjBalloonArray") TextureObjectBalloonArray;
				break;
			
			default:
				ShowMessage( "drawOGLPixmapAsTexture(): switch default encountered" );
				break;
			}
		}
	
	if ( cache->textureObject )
		cache->textureObject->draw( src, dst, rowLength, alphamap, targetAlpha, cache );
	else
		{
//		ShowMessage( "drawOGLPixmapAsTexture drawOGLPixmap()" );
			// cuz i want to know when this is called;
			// if it ever is, i probably need to fix something
		drawOGLPixmap( src, dst, rowLength, alphamap, cache->icImage.cliImage.GetBits() );
		}
}


void
TextureObjectSingle::draw(	const DTSRect& src,
							const DTSRect& dst,
							GLuint rowLength,
							IndexToAlphaMapID alphamap,
							GLfloat /*targetAlpha*/,
							ImageCache * cache )
{
	if ( not name )
		load( src, alphamap, cache );
	
	if ( name )
		{
		bindTexture( name );
		if ( gOGLManageTexturePriority )
			{
			priority = 1.0f;	// we're using it this frame, so maximize the priority
			glTexParameterf( gOGLTexTarget2D, GL_TEXTURE_PRIORITY, priority );
			}
		if ( kIndexToAlphaMapNone == alphamap )
			{
			disableBlending();	// faster than bothering with the index to alpha stuff
			disableAlphaTest();
			}
		else
			{
			enableBlending();
			enableAlphaTest();
			}
		enableTexturing();
		
		if ( cache->icImage.cliPictDef.pdFlags & kPictDefIsShadow )
			{
			enableStencilTest();
			setTextureEnvironmentMode( GL_MODULATE );
			GLfloat blendcolor[] = { 0.0f, 0.0f, 0.0f, GetShadowLevel() / 100.0f };
			glColor4fv( blendcolor );
			}
		else
			setTextureEnvironmentMode( GL_REPLACE );
		
		GLshort vertex[] =
			{
			dst.rectLeft, dst.rectTop,
			dst.rectRight, dst.rectTop,
			dst.rectRight, dst.rectBottom,
			dst.rectLeft, dst.rectBottom
			};
		
#ifdef OGL_USE_VERTEX_ARRAYS
		setVertexArray( 2, GL_SHORT, 0, vertex );
		setTexCoordArray( 2, GL_FLOAT, 0, texCoord );
		glDrawArrays( GL_QUADS, 0, 4 );
#else
		glBegin( GL_QUADS );
			for ( int i = 0; i < 4; ++i )
				{
				int ii = i + i;
				glTexCoord2fv( &texCoord[ ii ] );
				glVertex2sv( &vertex[ ii ] );
				}
		glEnd();
#endif	// OGL_USE_VERTEX_ARRAYS
		
		if ( cache->icImage.cliPictDef.pdFlags & kPictDefIsShadow )
			disableStencilTest();
		
		if ( gOGLManageTexturePriority )
			{
			GLint resident;
			glGetTexParameteriv( gOGLTexTarget2D, GL_TEXTURE_RESIDENT, &resident );
			if ( not resident )
				{
#if 1 && defined(DEBUG_VERSION)
				ShowMessage( "Nonresident texture detected..." );
#endif	// DEBUG_VERSION
				if ( deleteAllUnusedTextures() )
					{
#if 1 && defined(DEBUG_VERSION)
					ShowMessage( "deleteUnusedTextures() found something to delete" );
#endif	// DEBUG_VERSION
					}
				}
			}
#ifdef OGL_USE_VERTEX_ARRAYS
		// prevent dangling pointer to 'vertex', no longer in scope
		gOGLVertexArray = nullptr;
#endif
		}
	else
		{
		DShowMessage( "TextureObjectSingle::draw drawOGLPixmap()" );
		drawOGLPixmap( src, dst, rowLength, alphamap, cache->icImage.cliImage.GetBits() );
		}
}


bool
TextureObjectSingle::drawAsDropShadow(	const DTSRect& /*src*/,
										const DTSRect& /*dst*/,
										int /*hOffset*/,
										int /*vOffset*/,
										float /*shadowAlpha*/,
										ImageCache * /*cache*/ )
{
	ShowMessage( "TextureObjectSingle::drawAsDropShadow() not implemented" );
	return false;
}


bool
TextureObjectSingle::drawAsRotatedShadow(	const DTSRect& /*src*/,
											const DTSRect& /*dst*/,
											float /*angle*/,
											float /*scale*/,
											float /*shadowAlpha*/,
											ImageCache * /*cache*/ )
{
	ShowMessage( "TextureObjectSingle::drawAsRotatedShadow() not implemented" );
	return false;
}


void
TextureObjectSingle::load(	const DTSRect& /*src*/,
							IndexToAlphaMapID alphamap,
							ImageCache * cache )
{
	DTSRect allImagesBounds;
	cache->icImage.cliImage.GetBounds( &allImagesBounds );
	GLint newW, newH;
	
		// the default is GL_REPEAT, which isn't what we want,
		// and isn't supported by texture_rectangle,
		// but CL_CLAMP would require a border, and it isn't
		// obvious what the border color should be
		
		// and if I ever get around to implementing it,
		// "tiled" textures SHOULD be using GL_REPEAT
	GLint clamp;
	if ( gOGLHaveClampToEdge )
		clamp = GL_CLAMP_TO_EDGE;
	else
	if ( gOGLHaveTexRectExtension )
		clamp = GL_CLAMP;
	else
		clamp = GL_REPEAT;
	
	if ( loadTextureObject(
				name,	// reference
				0,		// hOffset
				0,		// vOffset
				allImagesBounds.rectRight - allImagesBounds.rectLeft,	// srcTexWidth
				allImagesBounds.rectBottom - allImagesBounds.rectTop,	// srcTexHeight
				&newW,			// dstTexWidth
				&newH,			// dstTexHeight
				clamp,
				cache,
				alphamap ) )
		{
		if ( gOGLHaveTexRectExtension )
			{
			invTexWidth = 1.0f;
			invTexHeight = 1.0f;
			}
		else
			{
			invTexWidth = 1.0f / newW;
			invTexHeight = 1.0f / newH;
			}
		
		texCoord[0] = texCoord[6] = allImagesBounds.rectLeft * invTexWidth;
		texCoord[2] = texCoord[4] = allImagesBounds.rectRight * invTexWidth;
		texCoord[1] = texCoord[3] = allImagesBounds.rectTop * invTexHeight;
		texCoord[5] = texCoord[7] = allImagesBounds.rectBottom * invTexHeight;
		}
}


void
TextureObjectNAnimated::draw(	const DTSRect& src,
								const DTSRect& dst,
								GLuint rowLength,
								IndexToAlphaMapID alphamap,
								GLfloat /*targetAlpha*/,
								ImageCache * cache )
{
	if ( not name )
		{
		int frames = cache->icImage.cliPictDef.pdNumFrames;
		priority = NEW_TAG("GLclampf") GLclampf[ frames ];
		if ( priority )
			{
			name = NEW_TAG("GLuint") GLuint[ frames ];
			if ( name )
				{
				totalFrames = frames;
				for ( uint i = 0; i < totalFrames; ++i )
					{
					name[i] = 0;
					priority[i] = 1.0f;
					}
				}
			else
				{
				delete[] priority;
				priority = nullptr;
				}
			}
		}
	if ( name )
		{
		int frameIndex = cache->icBox.rectTop / cache->icHeight;
		if ( not name[ frameIndex ] )
			load( src, alphamap, cache );
		
		if ( name[ frameIndex ] )
			{
			setTextureEnvironmentMode( GL_REPLACE );
			bindTexture( name[ frameIndex ] );
			if ( gOGLManageTexturePriority )
				{
					// we're using it this frame, so maximize the priority
				priority[frameIndex] = 1.0f;
				glTexParameterf( gOGLTexTarget2D, GL_TEXTURE_PRIORITY, priority[ frameIndex ] );
				}
			if ( kIndexToAlphaMapNone == alphamap )
				{
				disableBlending();
				disableAlphaTest();
				}
			else
				{
				enableBlending();
				enableAlphaTest();
				}
			enableTexturing();
			
			if ( cache->icImage.cliPictDef.pdFlags & kPictDefIsShadow )
				{
				enableStencilTest();
				setTextureEnvironmentMode( GL_MODULATE );
				GLfloat blendcolor[] = { 0.0f, 0.0f, 0.0f, GetShadowLevel() / 100.0f };
				glColor4fv( blendcolor );
				}
			else
				setTextureEnvironmentMode( GL_REPLACE );
			
			GLshort vertex[] =
				{
				dst.rectLeft, dst.rectTop,
				dst.rectRight, dst.rectTop,
				dst.rectRight, dst.rectBottom,
				dst.rectLeft, dst.rectBottom
				};

#ifdef OGL_USE_VERTEX_ARRAYS
			setVertexArray( 2, GL_SHORT, 0, vertex );
			setTexCoordArray( 2, GL_FLOAT, 0, texCoord );
			glDrawArrays( GL_QUADS, 0, 4 );
#else
			glBegin( GL_QUADS );
				for ( int i = 0; i < 4; ++i )
					{
					int ii = i + i;
					glTexCoord2fv( &texCoord[ii] );
					glVertex2sv( &vertex[ii] );
					}
			glEnd();
#endif	// OGL_USE_VERTEX_ARRAYS
			
			if ( cache->icImage.cliPictDef.pdFlags & kPictDefIsShadow )
				disableStencilTest();
			
			if ( gOGLManageTexturePriority )
				{
				GLint resident;
				glGetTexParameteriv( gOGLTexTarget2D, GL_TEXTURE_RESIDENT, &resident );
				if ( not resident )
					{
#if 1 && defined(DEBUG_VERSION)
					ShowMessage( "Nonresident texture detected..." );
#endif	// DEBUG_VERSION
					if ( deleteAllUnusedTextures() )
						{
#if 1 && defined(DEBUG_VERSION)
						ShowMessage( "deleteUnusedTextures() found something to delete" );
#endif	// DEBUG_VERSION
						}
					}
				}
#ifdef OGL_USE_VERTEX_ARRAYS
			// prevent dangling pointer to 'vertex', no longer in scope
			gOGLVertexArray = nullptr;
#endif
			}
		else
			{
			DShowMessage( "TextureObjectNAnimated::draw drawOGLPixmap()" );
			drawOGLPixmap( src, dst, rowLength, alphamap, cache->icImage.cliImage.GetBits() );
			}
		}
	else
		{
		DShowMessage( "TextureObjectNAnimated::draw drawOGLPixmap() ( ! name[frameIndex])" );
		drawOGLPixmap( src, dst, rowLength, alphamap, cache->icImage.cliImage.GetBits() );
		}
}


bool
TextureObjectNAnimated::drawAsDropShadow(	const DTSRect& /*src*/,
											const DTSRect& /*dst*/,
											int /*hOffset*/,
											int /*vOffset*/,
											float /*shadowAlpha*/,
											ImageCache * /*cache*/ )
{
	ShowMessage( "TextureObjectNAnimated::drawAsDropShadow() not implemented" );
	return false;
}


bool
TextureObjectNAnimated::drawAsRotatedShadow(	const DTSRect& /*src*/,
												const DTSRect& /*dst*/,
												float /*angle*/,
												float /*scale*/,
												float /*shadowAlpha*/,
												ImageCache * /*cache*/ )
{
	ShowMessage( "TextureObjectNAnimated::drawAsRotatedShadow() not implemented" );
	return false;
}


void
TextureObjectNAnimated::load(	const DTSRect& /*src*/,
								IndexToAlphaMapID alphamap,
								ImageCache * cache )
{
	if ( not name )
		{
		int frames = cache->icImage.cliPictDef.pdNumFrames;
		priority = NEW_TAG("GLclampf") GLclampf[ frames ];
		if ( priority )
			{
			name = NEW_TAG("GLuint") GLuint[ frames ];
			if ( name )
				{
				totalFrames = frames;
				for ( uint i = 0; i < totalFrames; ++i )
					{
					name[i] = 0;
					priority[i] = 1.0f;
					}
				}
			else
				{
				delete[] priority;
				priority = nullptr;
				}
			}
		}
	if ( name )
		{
		int frameIndex = cache->icBox.rectTop / cache->icHeight;
		if ( not name[ frameIndex ] )
			{
			DTSRect allImagesBounds;
			cache->icImage.cliImage.GetBounds( &allImagesBounds );
			GLint newW, newH;
				// the default is GL_REPEAT, which isn't what we want,
				// and isn't supported by texture_rectangle,
				// but CL_CLAMP would require a border, and it isn't
				// obvious what the border color should be
			
				// and if I ever get around to implementing it,
				// "tiled" textures SHOULD be using GL_REPEAT
			GLint clamp;
			if ( gOGLHaveClampToEdge )
				clamp = GL_CLAMP_TO_EDGE;
			else
			if ( gOGLHaveTexRectExtension )
				clamp = GL_CLAMP;
			else
				clamp = GL_REPEAT;
			
			if ( loadTextureObject(
						name[ frameIndex ],		// reference
						0,						// hOffset
						cache->icBox.rectTop,	// vOffset
						allImagesBounds.rectRight - allImagesBounds.rectLeft,
												// srcTexWidth
						cache->icHeight,		// srcTexHeight
						&newW,					// dstTexWidth
						&newH,					// dstTexHeight
						clamp,
						cache,
						alphamap ) )
				{
				if ( gOGLHaveTexRectExtension )
					{
					invTexWidth = 1.0f;
					invTexHeight = 1.0f;
					}
				else
					{
					invTexWidth = 1.0f / newW;
					invTexHeight = 1.0f / newH;
					}
				
					// this is getting called redundantly each time a new frame is loaded, but
					// the alternative is to create some kind of "firstTimeOnly" flag,
					// and that's just a different kind of waste, so which should i waste,
					// memory or cycles?
				texCoord[0] = texCoord[6] = allImagesBounds.rectLeft * invTexWidth;
				texCoord[2] = texCoord[4] = allImagesBounds.rectRight * invTexWidth;
				texCoord[1] = texCoord[3] = 0.0f;
				texCoord[5] = texCoord[7] = cache->icHeight * invTexHeight;
				}
			}
		}
}


void
TextureObjectMobileArray::draw(	const DTSRect& src,
								const DTSRect& dst,
								GLuint rowLength,
								IndexToAlphaMapID alphamap,
								GLfloat /*targetAlpha*/,
								ImageCache * cache )
{
	DTSRect allImagesBounds;
	cache->icImage.cliImage.GetBounds( &allImagesBounds );
	int mobileSize = allImagesBounds.rectRight >> 4;
	if ( not mobileSize )
//		return;
		mobileSize = 1;
	
	int frameVIndex = src.rectTop / mobileSize;	// note that cust color offset rounds out
	int frameHIndex = src.rectLeft / mobileSize;
	
	if ( not name[frameHIndex][frameVIndex] )
		load( src, alphamap, cache );
		
	if ( name[frameHIndex][frameVIndex] )
		{
		setTextureEnvironmentMode( GL_REPLACE );
		bindTexture( name[frameHIndex][frameVIndex] );
		if ( gOGLManageTexturePriority )
			{
			// we're using it this frame, so maximize the priority
			priority[frameHIndex][frameVIndex] = 1.0f;
			glTexParameterf( gOGLTexTarget2D, GL_TEXTURE_PRIORITY,
				priority[frameHIndex][frameVIndex] );
			}
		if ( kIndexToAlphaMapNone == alphamap )
			{
			disableBlending();	// faster than bothering with the index to alpha stuff
			disableAlphaTest();
			}
		else
			{
			enableBlending();
			enableAlphaTest();
			}
		enableTexturing();
		
		GLshort vertex[] =
			{
			dst.rectLeft, dst.rectTop,
			dst.rectRight, dst.rectTop,
			dst.rectRight, dst.rectBottom,
			dst.rectLeft, dst.rectBottom
			};
		
#ifdef OGL_USE_VERTEX_ARRAYS
		setVertexArray( 2, GL_SHORT, 0, vertex );
		setTexCoordArray( 2, GL_FLOAT, 0, texCoord );
		glDrawArrays( GL_QUADS, 0, 4 );
#else
		glBegin( GL_QUADS );
			for ( int i = 0; i < 4; ++i )
				{
				int ii = i + i;
				glTexCoord2fv( &texCoord[ii] );
				glVertex2sv( &vertex[ii] );
				}
		glEnd();
#endif	// OGL_USE_VERTEX_ARRAYS
		
		if ( gOGLManageTexturePriority )
			{
			GLint resident;
			glGetTexParameteriv( gOGLTexTarget2D, GL_TEXTURE_RESIDENT, &resident );
			if ( not resident )
				{
#if 1 && defined(DEBUG_VERSION)
				DShowMessage( "Nonresident texture detected..." );
#endif	// DEBUG_VERSION
				if ( deleteAllUnusedTextures() )
					{
#if 1 && defined(DEBUG_VERSION)
					DShowMessage( "deleteUnusedTextures() found something to delete" );
#endif	// DEBUG_VERSION
					}
				}
			}
#ifdef OGL_USE_VERTEX_ARRAYS
		// prevent dangling pointer to 'vertex', no longer in scope
		gOGLVertexArray = nullptr;
#endif
		}
	else
		{
		DShowMessage( "TextureObjectMobileArray::draw drawOGLPixmap()" );
		drawOGLPixmap( src, dst, rowLength, alphamap, cache->icImage.cliImage.GetBits() );
		}
}


bool
TextureObjectMobileArray::drawAsDropShadow(	const DTSRect& src,
											const DTSRect& dstIn,
											int hOffset,
											int vOffset,
											float shadowAlpha,
											ImageCache * cache )
{
	DTSRect allImagesBounds;
	cache->icImage.cliImage.GetBounds( &allImagesBounds );
	int mobileSize = allImagesBounds.rectRight >> 4;
	if ( not mobileSize )
		mobileSize = 1;		// return false;
	
	int frameVIndex = src.rectTop / mobileSize;	// note that cust color offset rounds out
	int frameHIndex = src.rectLeft / mobileSize;
	
	if ( not name[frameHIndex][frameVIndex] )
		{
		int blitter = (cache->icImage.cliPictDef.pdFlags & kPictDefBlendMask);
		IndexToAlphaMapID alphamap;
		switch ( blitter )
			{
			case kPictDef75Blend: alphamap = kIndexToAlphaMapAlpha25TransparentZero; break;
			case kPictDef50Blend: alphamap = kIndexToAlphaMapAlpha50TransparentZero; break;
			case kPictDef25Blend: alphamap = kIndexToAlphaMapAlpha75TransparentZero; break;

			default:
			case kPictDefNoBlend: alphamap = kIndexToAlphaMapAlpha100TransparentZero; break;
			}
		load( src, alphamap, cache );
		}
	
	if ( name[frameHIndex][frameVIndex] )
		{
		DTSRect dst = dstIn;
		dst.Offset( hOffset, vOffset );
		bindTexture( name[frameHIndex][frameVIndex] );
		if ( gOGLManageTexturePriority )
			{
				// we're using it this frame, so maximize the priority
			priority[frameHIndex][frameVIndex] = 1.0f;
			glTexParameterf( gOGLTexTarget2D, GL_TEXTURE_PRIORITY,
				priority[frameHIndex][frameVIndex] );
			}
		
		enableBlending();
		enableAlphaTest();
		enableTexturing();
		enableStencilTest();
		
		setTextureEnvironmentMode( GL_MODULATE );
		
		glTexParameteri( gOGLTexTarget2D, GL_TEXTURE_MAG_FILTER, GL_LINEAR );
			// this IS stored in the object
		glTexParameteri( gOGLTexTarget2D, GL_TEXTURE_MIN_FILTER, GL_LINEAR );
			// this IS stored in the object
		
		GLfloat blendcolor[] = { 0.0f, 0.0f, 0.0f, shadowAlpha };
		glColor4fv( blendcolor );
		
		GLshort vertex[] = {
			dst.rectLeft, dst.rectTop,
			dst.rectRight, dst.rectTop,
			dst.rectRight, dst.rectBottom,
			dst.rectLeft, dst.rectBottom
			};
		
#ifdef OGL_USE_VERTEX_ARRAYS
		setVertexArray( 2, GL_SHORT, 0, vertex );
		setTexCoordArray( 2, GL_FLOAT, 0, texCoord );
		glDrawArrays( GL_QUADS, 0, 4 );
#else
		glBegin( GL_QUADS );
			for ( int i = 0; i < 4; ++i )
				{
				int ii = i + i;
				glTexCoord2fv( &texCoord[ii] );
				glVertex2sv( &vertex[ii] );
				}
		glEnd();
#endif	// OGL_USE_VERTEX_ARRAYS
		
		if ( gOGLManageTexturePriority )
			{
			GLint resident;
			glGetTexParameteriv( gOGLTexTarget2D, GL_TEXTURE_RESIDENT, &resident );
			if ( not resident )
				{
#if 1 && defined(DEBUG_VERSION)
				DShowMessage( "Nonresident texture detected..." );
#endif	// DEBUG_VERSION
				if ( deleteAllUnusedTextures() )
					{
#if 1 && defined(DEBUG_VERSION)
					DShowMessage( "deleteUnusedTextures() found something to delete" );
#endif	// DEBUG_VERSION
					}
				}
			}
		
		// what would I save by creating "shadow texture" objects?
		glTexParameteri( gOGLTexTarget2D, GL_TEXTURE_MAG_FILTER, GL_NEAREST );
			// this IS stored in the object
		glTexParameteri( gOGLTexTarget2D, GL_TEXTURE_MIN_FILTER, GL_NEAREST );
			// this IS stored in the object
		
#ifdef OGL_USE_VERTEX_ARRAYS
		// prevent dangling pointer to 'vertex', no longer in scope
		gOGLVertexArray = nullptr;
#endif
		
		return true;
		}
	
	return false;
}


bool
TextureObjectMobileArray::drawAsRotatedShadow(	const DTSRect& src,
												const DTSRect& dstIn,
												float angle,
												float scale,
												float shadowAlpha,
												ImageCache * cache )
{
	DTSRect allImagesBounds;
	cache->icImage.cliImage.GetBounds( &allImagesBounds );
	int mobileSize = allImagesBounds.rectRight >> 4;
	if ( not mobileSize )
		mobileSize = 1;		// return false;
	
	int frameVIndex = src.rectTop / mobileSize;	// note that cust color offset rounds out
	int frameHIndex = src.rectLeft / mobileSize;
	
	if ( not name[frameHIndex][frameVIndex] )
		{
		int blitter = (cache->icImage.cliPictDef.pdFlags & kPictDefBlendMask);
		IndexToAlphaMapID alphamap;
		switch ( blitter )
			{
			case kPictDef75Blend: alphamap = kIndexToAlphaMapAlpha25TransparentZero; break;
			case kPictDef50Blend: alphamap = kIndexToAlphaMapAlpha50TransparentZero; break;
			case kPictDef25Blend: alphamap = kIndexToAlphaMapAlpha75TransparentZero; break;

			default:
			case kPictDefNoBlend: alphamap = kIndexToAlphaMapAlpha100TransparentZero; break;
			}
		load( src, alphamap, cache );
		}
	
	if ( name[frameHIndex][frameVIndex] )
		{
		const int baseMargin = 2;
		GLfloat centerH = float(dstIn.rectLeft + dstIn.rectRight) * 0.5f;
		GLfloat centerV = dstIn.rectBottom - baseMargin;
		DTSRect dst = dstIn;
		dst.Offset( -centerH, -centerV );
		dst.rectTop *= scale;
		bindTexture( name[frameHIndex][frameVIndex] );
		if ( gOGLManageTexturePriority )
			{
				// we're using it this frame, so maximize the priority
			priority[frameHIndex][frameVIndex] = 1.0f;
			glTexParameterf( gOGLTexTarget2D, GL_TEXTURE_PRIORITY,
				priority[frameHIndex][frameVIndex] );
			}
		
		enableBlending();
		enableAlphaTest();
		enableTexturing();
		enableStencilTest();
		
		setTextureEnvironmentMode( GL_MODULATE );	// this is NOT stored in the object
		GLfloat blendcolor[] = { 0.0f, 0.0f, 0.0f, shadowAlpha };
		glColor4fv( blendcolor );
		
		GLshort vertex[] = {
	// the original rotation code must have a flip in it that I didn't see.
	// a rotation is not a flip, so we need to swap the left and right vertexes
	// (vertices?  verticii?)
			dst.rectRight, dst.rectTop,
			dst.rectLeft, dst.rectTop,
			dst.rectLeft, dst.rectBottom,
			dst.rectRight, dst.rectBottom
			};
		
#ifdef OGL_USE_VERTEX_ARRAYS
		setVertexArray( 2, GL_SHORT, 0, vertex );
		setTexCoordArray( 2, GL_FLOAT, 0, texCoord );
#endif	// OGL_USE_VERTEX_ARRAYS
		
		glTexParameteri( gOGLTexTarget2D, GL_TEXTURE_MAG_FILTER, GL_LINEAR );
			// this IS stored in the object
		glTexParameteri( gOGLTexTarget2D, GL_TEXTURE_MIN_FILTER, GL_LINEAR );
			// this IS stored in the object
		
		glPushMatrix();
			glTranslatef( centerH, centerV, 0.0f );
			glRotatef( angle + 90.0f, 0.0f, 0.0f, -1.0f );
#ifdef OGL_USE_VERTEX_ARRAYS
			glDrawArrays( GL_QUADS, 0, 4 );
#else
			glBegin( GL_QUADS );
				for ( int i = 0; i < 4; ++i )
					{
					int ii = i + i;
					glTexCoord2fv( &texCoord[ii] );
					glVertex2sv( &vertex[ii] );
					}
			glEnd();
#endif	// OGL_USE_VERTEX_ARRAYS
		
		glPopMatrix();
		
		if ( gOGLManageTexturePriority )
			{
			GLint resident;
			glGetTexParameteriv( gOGLTexTarget2D, GL_TEXTURE_RESIDENT, &resident );
			if ( not resident )
				{
#if 1 && defined(DEBUG_VERSION)
				DShowMessage( "Nonresident texture detected..." );
#endif	// DEBUG_VERSION
				if ( deleteAllUnusedTextures() )
					{
#if 1 && defined(DEBUG_VERSION)
					DShowMessage( "deleteUnusedTextures() found something to delete" );
#endif	// DEBUG_VERSION
					}
				}
			}
		
		// what would I save by creating "shadow texture" objects?
		glTexParameteri( gOGLTexTarget2D, GL_TEXTURE_MAG_FILTER, GL_NEAREST );
			// this IS stored in the object
		glTexParameteri( gOGLTexTarget2D, GL_TEXTURE_MIN_FILTER, GL_NEAREST );
			// this IS stored in the object
		
#ifdef OGL_USE_VERTEX_ARRAYS
		// prevent dangling pointer to 'vertex', no longer in scope
		gOGLVertexArray = nullptr;
#endif
		return true;
		}
	
	return false;
}


void
TextureObjectMobileArray::load(	const DTSRect& src,
								IndexToAlphaMapID alphamap,
								ImageCache * cache )
{
//	DShowMessage( "TextureObjectMobileArray::load ID %d, IndexToAlphaMapID %d",
//		cache->icImage.cliPictDefID, alphamap );
	DTSRect allImagesBounds;
	cache->icImage.cliImage.GetBounds( &allImagesBounds );
	int mobileSize = allImagesBounds.rectRight >> 4;
	if ( not mobileSize )
		mobileSize = 1;		// return;
	
	int frameVIndex = src.rectTop / mobileSize;	// note that cust color offset rounds out
	int frameHIndex = src.rectLeft / mobileSize;
	
	if ( not name[frameHIndex][frameVIndex] )
		{
		GLint newSize;
			// the default is GL_REPEAT, which isn't what we want,
			// and isn't supported by texture_rectangle,
			// but CL_CLAMP would require a border, and it isn't
			// obvious what the border color should be
			
			// and if I ever get around to implementing it,
			// "tiled" textures SHOULD be using GL_REPEAT
		GLint clamp;
		if ( gOGLHaveClampToEdge )
			clamp = GL_CLAMP_TO_EDGE;
		else
		if ( gOGLHaveTexRectExtension )
			clamp = GL_CLAMP;
		else
			clamp = GL_REPEAT;
		
		if ( loadTextureObject(	name[frameHIndex][frameVIndex],	// reference
								src.rectLeft,	// hOffset
								src.rectTop,	// vOffset
								mobileSize,		// srcTexWidth
								mobileSize,		// srcTexHeight
								&newSize,		// dstTexWidth
								&newSize,		// dstTexHeight
								clamp,
								cache,
								alphamap ) )
			{
			if ( gOGLHaveTexRectExtension )
				invTexSize = 1.0f;
			else
				invTexSize = 1.0f / newSize;
			
				// this is getting called redundantly each time a new frame is loaded, but the
				// alternative is to create some kind of "firstTimeOnly" flag, and that's just a
				// different kind of waste, so which should i waste, memory or cycles?
	
			texCoord[0] = texCoord[6] = 0.0f;
			texCoord[2] = texCoord[4] = mobileSize * invTexSize;
			texCoord[1] = texCoord[3] = 0.0f;
			texCoord[5] = texCoord[7] = mobileSize * invTexSize;
			}
		}
}


void
TextureObjectBalloonArray::draw(	const DTSRect& src,
									const DTSRect& dst,
									GLuint rowLength,
									IndexToAlphaMapID alphamap,
									GLfloat targetAlpha,
									ImageCache * cache )
{
	DTSRect allImagesBounds;
	cache->icImage.cliImage.GetBounds( &allImagesBounds );
	int balloonVSize = allImagesBounds.rectBottom >> 2;
	int balloonHSize = allImagesBounds.rectRight / 6;
	int frameVIndex = src.rectTop / balloonVSize;	// note that cust color offset rounds out
	int frameHIndex = src.rectLeft / balloonHSize;
	
	if ( not name[frameHIndex][frameVIndex] )
		load( src, kIndexToAlphaMapAlpha100TransparentZero, cache );
	
	if ( name[frameHIndex][frameVIndex] )
		{
		bindTexture( name[frameHIndex][frameVIndex] );
		if ( gOGLManageTexturePriority )
			{
				// we're using it this frame, so maximize the priority
			priority[frameHIndex][frameVIndex] = 1.0f;
			glTexParameterf( gOGLTexTarget2D, GL_TEXTURE_PRIORITY,
				priority[frameHIndex][frameVIndex] );
			}
		if ( kIndexToAlphaMapNone == alphamap )
			{
			disableBlending();
			disableAlphaTest();
			}
		else
			{
			enableBlending();
			enableAlphaTest();
			}
		
		setTextureEnvironmentMode( GL_MODULATE );
		GLfloat blendcolor[] = { 1.0f, 1.0f, 1.0f, 1.0f };
		switch ( alphamap )
			{
			case kIndexToAlphaMapAlpha25TransparentZero: blendcolor[3] = 0.25f; break;
			case kIndexToAlphaMapAlpha50TransparentZero: blendcolor[3] = 0.50f; break;
			case kIndexToAlphaMapAlpha75TransparentZero: blendcolor[3] = 0.75f; break;

			default:
			case kIndexToAlphaMapAlpha100TransparentZero:
				// no change
				break;
			}
		if ( targetAlpha < 1.0f )
			blendcolor[3] *= targetAlpha;
		glColor4fv( blendcolor );
		
		enableTexturing();
		
		GLshort vertex[] = {
			dst.rectLeft, dst.rectTop,
			dst.rectRight, dst.rectTop,
			dst.rectRight, dst.rectBottom,
			dst.rectLeft, dst.rectBottom
			};
		
#ifdef OGL_USE_VERTEX_ARRAYS
		setVertexArray( 2, GL_SHORT, 0, vertex );
		setTexCoordArray( 2, GL_FLOAT, 0, texCoord );
		glDrawArrays( GL_QUADS, 0, 4 );
#else
		glBegin( GL_QUADS );
			for ( int i = 0; i < 4; ++i )
				{
				int ii = i + i;
				glTexCoord2fv( &texCoord[ii] );
				glVertex2sv( &vertex[ii] );
				}
		glEnd();
#endif	// OGL_USE_VERTEX_ARRAYS
		
		if ( gOGLManageTexturePriority )
			{
			GLint resident;
			glGetTexParameteriv( gOGLTexTarget2D, GL_TEXTURE_RESIDENT, &resident );
			if ( not resident )
				{
#if 1 && defined(DEBUG_VERSION)
				DShowMessage( "Nonresident texture detected..." );
#endif	// DEBUG_VERSION
				if ( deleteAllUnusedTextures() )
					{
#if 1 && defined(DEBUG_VERSION)
					DShowMessage( "deleteUnusedTextures() found something to delete" );
#endif	// DEBUG_VERSION
					}
				}
			}

			// or should i make the text draw function responsible for setting it's own state?
		disableTexturing();
		
#ifdef OGL_USE_VERTEX_ARRAYS
		// prevent dangling pointer to 'vertex', no longer in scope
		gOGLVertexArray = nullptr;
#endif
		}
	else
		{
		DShowMessage( "TextureObjectBalloonArray::draw drawOGLPixmap()" );
		drawOGLPixmap( src, dst, rowLength, alphamap, cache->icImage.cliImage.GetBits() );
		}
}


bool
TextureObjectBalloonArray::drawAsDropShadow(	const DTSRect& /*src*/,
												const DTSRect& /*dst*/,
												int /*hOffset*/,
												int /*vOffset*/,
												float /*shadowAlpha*/,
												ImageCache * /*cache*/ )
{
	// I don't expect to EVER call this
	ShowMessage( "TextureObjectBalloonArray::drawAsDropShadow() was called unexpectedly" );
	return false;
}


bool
TextureObjectBalloonArray::drawAsRotatedShadow(	const DTSRect& /*src*/,
												const DTSRect& /*dst*/,
												float /*angle*/,
												float /*scale*/,
												float /*shadowAlpha*/,
												ImageCache * /*cache*/ )
{
	ShowMessage( "TextureObjectBalloonArray::drawAsRotatedShadow() was called unexpectedly" );
	return false;
}


void
TextureObjectBalloonArray::load(	const DTSRect& src,
									IndexToAlphaMapID alphamap,
									ImageCache * cache )
{
//	DShowMessage( "TextureObjectBalloonArray::load ID %d, IndexToAlphaMapID %d",
//		cache->icImage.cliPictDefID, alphamap );
	DTSRect allImagesBounds;
	cache->icImage.cliImage.GetBounds( &allImagesBounds );
	int balloonVSize = allImagesBounds.rectBottom >> 2;
	int balloonHSize = allImagesBounds.rectRight / 6;
	int frameVIndex = src.rectTop / balloonVSize;	// note that cust color offset rounds out
	int frameHIndex = src.rectLeft / balloonHSize;
	
	if ( not name[frameHIndex][frameVIndex] )
		{
		GLint newW, newH;
			// the default is GL_REPEAT, which isn't what we want,
			// and isn't supported by texture_rectangle,
			// but CL_CLAMP would require a border, and it isn't
			// obvious what the border color should be
			
			// and if I ever get around to implementing it,
			// "tiled" textures SHOULD be using GL_REPEAT
		GLint clamp;
		if ( gOGLHaveClampToEdge )
			clamp = GL_CLAMP_TO_EDGE;
		else
		if( gOGLHaveTexRectExtension )
			clamp = GL_CLAMP;
		else
			clamp = GL_REPEAT;
		
		if ( loadTextureObject(	name[frameHIndex][frameVIndex],	// reference
								src.rectLeft,	// hOffset
								src.rectTop,	// vOffset
								balloonHSize,	// srcTexWidth
								balloonVSize,	// srcTexHeight
								&newW,			// dstTexWidth
								&newH,			// dstTexHeight
								clamp,
								cache,
								alphamap ) )
			{
			invTexWidth = 1.0f / newW;
			invTexHeight = 1.0f / newH;
			if ( gOGLHaveTexRectExtension )
				{
				invTexWidth = 1.0f;
				invTexHeight = 1.0f;
				}
			else
				{
				invTexWidth = 1.0f / newW;
				invTexHeight = 1.0f / newH;
				}
			
				// this is getting called redundantly each time a new frame is loaded, but the
				// alternative is to create some kind of "firstTimeOnly" flag, and that's just a
				// different kind of waste, so which should i waste, memory or cycles?
			
			texCoord[0] = texCoord[6] = 0.0f;
			texCoord[2] = texCoord[4] = balloonHSize * invTexWidth;
			texCoord[1] = texCoord[3] = 0.0f;
			texCoord[5] = texCoord[7] = balloonVSize * invTexHeight;
			}
		}
}


bool
TextureObject::loadTextureObject(
	GLuint &			textureObjectName,
	GLint				hOffset,
	GLint				vOffset,
	GLint				srcTexWidth,
	GLint				srcTexHeight,
	GLint *				dstTexWidth,
	GLint *				dstTexHeight,
	GLint				clamp,
	ImageCache *		cache,
	IndexToAlphaMapID	alphamap )
{
	bool returnVal = false;
	
	*dstTexWidth = 1;
	while ( *dstTexWidth < srcTexWidth )
		*dstTexWidth = *dstTexWidth << 1;
	
		// note:  for mobiles, dstTexWidth == dstTexHeight and srcTexWidth == srcTexHeight
		// so no need to (re)calculate *dstTextHeight
	if ( dstTexWidth == dstTexHeight && srcTexWidth == srcTexHeight )
		*dstTexHeight = *dstTexWidth;
	else
		{
		*dstTexHeight = 1;
		while ( *dstTexHeight < srcTexHeight )
			*dstTexHeight = *dstTexHeight << 1;
		}
	
	GLint maxtexsize;
	bool oversize = false;
	if ( gOGLHaveTexRectExtension )
		{
		glGetIntegerv( GL_MAX_RECTANGLE_TEXTURE_SIZE_EXT, &maxtexsize );
		if ( maxtexsize < srcTexWidth || maxtexsize < srcTexHeight )
			oversize = true;
		}
	else
		{
		glGetIntegerv( GL_MAX_TEXTURE_SIZE, &maxtexsize );
		if ( maxtexsize < *dstTexWidth || maxtexsize < *dstTexHeight )
		oversize = true;
		}
	
	if ( oversize )
		{
		if ( gOGLHaveTexRectExtension )
			ShowMessage( "MAYDAY! texturesize > GL_MAX_RECTANGLE_TEXTURE_SIZE_EXT: "
						 "%d w X %d h (%d max)",
						 (int) srcTexWidth, (int) srcTexHeight, (int) maxtexsize );
		else
			ShowMessage( "MAYDAY! texturesize > GL_MAX_TEXTURE_SIZE: "
						  "%d w X %d h -> %d w X %d h (%d max)",
				(int) srcTexWidth, (int) srcTexHeight, (int) *dstTexWidth,
				(int) *dstTexHeight, (int) maxtexsize );
		}
	else
		{
		// need a temp object, or the DecrementTextureObjectPriorities/deleteAllUnusedTextures
		// pair could delete it while i'm using it
		GLuint tempTextureObjectName;
		glGenTextures( 1, &tempTextureObjectName );
		if ( tempTextureObjectName )
			{
			bindTexture( tempTextureObjectName );
			
			GLint internalFormat = (kIndexToAlphaMapNone == alphamap)
				? GL_RGB : GL_RGBA;	// formerly used only GL_RGBA
			if ( gOGLRendererRage128 || gOGLRendererSmallTextureMemory )
				{
					// some Rage 128 (and all earlier) have teeny vram, so use 16bit textures
				if ( GL_RGB == internalFormat )
					internalFormat = GL_RGB5;		// GL_RGB4 ?
				if ( GL_RGBA == internalFormat )
					internalFormat = GL_RGBA4;	// GL_RGB5_A1 ?
				}
			
			GLenum format = GL_COLOR_INDEX, type = GL_UNSIGNED_BYTE;
			
				// ...except that texture rectangles don't support color index
				// UNLESS paletted textures are NOT available...
			if ( gOGLHaveTexRectExtension && gOGLHavePalettedTextureExtension )
				{
				if ( gOGLHaveBGRA && gOGLHavePackedPixels )
					{
						// this is the "native" format for carbon/quickdraw,
						// and the drivers are optimized for it
						// the speed gain is probably unmeasurable, but what the heck...
					format = GL_BGRA;
					type = GL_UNSIGNED_INT_8_8_8_8_REV;
					}
				else
					{
					format = GL_RGBA;
					type = GL_UNSIGNED_BYTE;
					}
				}
				if ( gOGLBrokenTexSubImage_ColorIndex )
					{
					format = GL_RGBA;
					type = GL_UNSIGNED_BYTE;
					}
			
					// this does NOT allocate memory:
					// it checks to see if we are within certain limits
			GLint testwidth;
			bool tryAgain = false;
			
			do {
				if ( gOGLHaveTexRectExtension )
					glTexImage2D(	GL_PROXY_TEXTURE_RECTANGLE_EXT, 0, internalFormat,
									srcTexWidth, srcTexHeight, 0,
									format, type, nullptr );
				else
					glTexImage2D(	GL_PROXY_TEXTURE_2D, 0, internalFormat,
									*dstTexWidth, *dstTexHeight, 0,
									format, type, nullptr );
				
#if 0
if ( ShowOpenGLErrors() )
	{
	if ( gOGLHaveTexRectExtension )
		{
		DShowMessage( "maxtexsize %d", maxtexsize );
		DShowMessage( "%d %d", srcTexWidth, srcTexHeight );
		}
	else
		DShowMessage( "%d %d", *dstTexWidth, *dstTexHeight );
	}
#endif  // 0
				glGetTexLevelParameteriv( gOGLProxyTexTarget2D, 0, GL_TEXTURE_WIDTH, &testwidth );
				if ( not testwidth )
					{
					tryAgain = deleteAllUnusedTextures();
					DecrementTextureObjectPriorities();
#ifdef DEBUG_VERSION
					if ( tryAgain )
						ShowMessage( "No room for texture; "
							"deleting unused textures and trying again..." );
					else
						ShowMessage( "No room for texture; "
							"no other textures to delete, giving up..." );
#endif	// DEBUG_VERSION
					}
				} while ( not testwidth && tryAgain );
					// change to deleteFirstUnusedTexture when available?
			
			if ( testwidth )
				{
				textureObjectName = tempTextureObjectName;
					// strictly, i don't need this if using GL_NEAREST
					// but I might want to change someday
				glTexParameteri( gOGLTexTarget2D, GL_TEXTURE_WRAP_S, clamp );
				glTexParameteri( gOGLTexTarget2D, GL_TEXTURE_WRAP_T, clamp );
				glTexParameteri( gOGLTexTarget2D, GL_TEXTURE_MAG_FILTER, GL_NEAREST );
				glTexParameteri( gOGLTexTarget2D, GL_TEXTURE_MIN_FILTER, GL_NEAREST );
				
				if ( gOGLHaveTexRectExtension )
					// this actually allocates memory, which might not be available
					glTexImage2D(	GL_TEXTURE_RECTANGLE_EXT, 0, internalFormat,
									srcTexWidth, srcTexHeight, 0,
									format, type, nullptr );
				else
					// this actually allocates memory, which might not be available
					glTexImage2D(	GL_TEXTURE_2D, 0, internalFormat,
									*dstTexWidth, *dstTexHeight, 0,
									format, type, nullptr );
				
				if ( not ShowOpenGLErrors() ) // the most likely error is out-of-memory
					{
					if ( not gOGLBrokenTexSubImage_ColorIndex
					&&   not ( gOGLHaveTexRectExtension && gOGLHavePalettedTextureExtension ) )
						{
						setIndexToAlphaMap( alphamap );
						if ( vOffset )
							glPixelStorei( GL_UNPACK_SKIP_ROWS, vOffset );
						
						if ( hOffset )
							glPixelStorei( GL_UNPACK_SKIP_PIXELS, hOffset );
						
						if ( srcTexWidth != cache->icImage.cliImage.GetRowBytes() )
							glPixelStorei( GL_UNPACK_ROW_LENGTH,
								cache->icImage.cliImage.GetRowBytes() );
						
//						glPixelStorei( GL_UNPACK_ALIGNMENT, 1 );
							// only some systems do anything with this
						
						glTexSubImage2D(	gOGLTexTarget2D, 0, 0, 0,
											srcTexWidth,
											srcTexHeight,
											GL_COLOR_INDEX, GL_UNSIGNED_BYTE,
											cache->icImage.cliImage.GetBits() );
						
						if ( not ShowOpenGLErrors() )	// the most likely error is out-of-memory
							{
													// but OS9 has a general bug with this,
													// hence the version check above
							returnVal = true;
							}
						if ( vOffset )
							glPixelStorei( GL_UNPACK_SKIP_ROWS, 0 );
						
						if ( hOffset )
							glPixelStorei( GL_UNPACK_SKIP_PIXELS, 0 );
						
						if ( srcTexWidth != cache->icImage.cliImage.GetRowBytes() )
							glPixelStorei( GL_UNPACK_ROW_LENGTH, 0 );
						
//							glPixelStorei( GL_UNPACK_ALIGNMENT, 4 );	// default
						}
					else
						{
// since this is now a "normal" codepath for OSX,
// i should probably look into keeping this memory around between calls
// to avoid the allocation overhead

#define USE_TEMP_HANDLES	 0

#if ! USE_TEMP_HANDLES
// the DT way
						GLubyte * pixels = NEW_TAG("temppixels")
							GLubyte[ (*dstTexWidth) * (*dstTexHeight) * 4 ];
#else	// temp memory
						GLubyte * pixels = nullptr;
						OSStatus resultCode;
						Handle pixelsHandle = NewHandle(
							sizeof(GLubyte) * (*dstTexWidth) * (*dstTexHeight) * 4 );
						if ( not pixelsHandle )
							{
							resultCode = MemError();
							ShowMessage( "Could not allocate memory, error code: %d",
								int(resultCode) );
							}
						else
							{
							HLock( pixelsHandle );
							resultCode = MemError();
							if ( resultCode != noErr )
								{
								ShowMessage( "Could not lock memory, error code: %d",
									int(resultCode) );
								DisposeHandle( pixelsHandle );
								pixelsHandle = nullptr;
								}
							else
								pixels = *reinterpret_cast<GLubyte **>( pixelsHandle );
							}
#endif  // ! USE_TEMP_HANDLES

// later:  cache GetRowBytes(), GetBits(), etc
						if ( pixels )
							{
								// GL_RGBA and GL_BGRA are the only two possibilities here
							int r, g, b, a;	// index/offset into a 4 byte color
							
							if ( GL_BGRA == format )
								{
									// recall that type is GL_UNSIGNED_INT_8_8_8_8_REV
									// so we need to reverse the order of the components
								r = 1;
								g = 2;
								b = 3;
								a = 0;
								}
							else
								{
									// GL_RGBA
								r = 0;
								g = 1;
								b = 2;
								a = 3;
								}
							
							int irb = cache->icImage.cliImage.GetRowBytes();
							const GLubyte * indexpixel = 
								(GLubyte *) cache->icImage.cliImage.GetBits();
							
							for ( int i = 0; i < srcTexHeight; ++i )
								{
								int srcOffset = (i + vOffset) * irb + hOffset;
								for ( int j = 0; j < srcTexWidth; ++j )
									{
									GLubyte indexcolor = indexpixel[ srcOffset ];
									++srcOffset;
									
									int dstOff = 4 * (i * srcTexWidth + j);
									pixels[ dstOff + r ] = gIndexToRedMap  [ indexcolor ];
									pixels[ dstOff + g ] = gIndexToGreenMap[ indexcolor ];
									pixels[ dstOff + b ] = gIndexToBlueMap [ indexcolor ];
									if ( kIndexToAlphaMapNone == alphamap )
										pixels[ dstOff + a ] = 255;
									else
										pixels[ dstOff + a ] =
											IndexToAlphaMap[alphamap][indexcolor] >> 8;
									}
								}
							
//							glPixelStorei( GL_UNPACK_ALIGNMENT, 1 );	
								// only some systems do anything with this
							
							glTexSubImage2D( gOGLTexTarget2D, 0, 0, 0,
											srcTexWidth,
											srcTexHeight,
											format, type,
											pixels );
							
//							glPixelStorei( GL_UNPACK_ALIGNMENT, 4 );	// default
							
							if ( not ShowOpenGLErrors() ) // the most likely error is out-of-memory
								returnVal = true;
							
#if ! USE_TEMP_HANDLES	// the DT way
							delete[] pixels;
#else	// temp memory
							HUnlock( pixelsHandle );
							DisposeHandle( pixelsHandle );
#endif
							}	// pixels
						}	// gOGLHaveTexRectExtension ...
					} // ! ShowOpenGLErrors()
				}	// testwidth
		
		if ( returnVal )
			{
#ifdef OGL_SHOW_DRAWTIME
			++textureCount;
#endif	// OGL_SHOW_DRAWTIME
			
#ifdef DEBUG_VERSION
# if 0	// bytesPerPixels is unused unless DShowMessage() isn't a no-op
			int bytesPerPixels;
			switch ( internalFormat )
				{
				case GL_RGBA:
				case GL_BGRA:
					bytesPerPixels = 4;
					break;

				case GL_RGB:
					bytesPerPixels = 3;
					break;

				case GL_RGB4:
				case GL_RGB5:
				case GL_RGBA4:
				case GL_RGB5_A1:
					bytesPerPixels = 2;
					break;

				default:
					bytesPerPixels = 1;
					break;
				}
# endif  // 0
			
			if ( gOGLHaveTexRectExtension )
				{
				if (
					(gLargestTextureDimension < (uint) srcTexWidth)
						||
					(gLargestTextureDimension < (uint) srcTexHeight)
						||
					((int) gLargestTexturePixels < srcTexWidth * srcTexHeight) )
					{
					if ( (int) gLargestTextureDimension < srcTexWidth )
						gLargestTextureDimension = srcTexWidth;
					
					if ( (int) gLargestTextureDimension < srcTexHeight )
						gLargestTextureDimension = srcTexHeight;
					
					if ( (int) gLargestTexturePixels < srcTexWidth * srcTexHeight )
						gLargestTexturePixels = srcTexWidth * srcTexHeight;
					
					DShowMessage( "Largest texture sizes: "
						"%d length, %d pixels (%d bytes) (%d w X %d h)",
						gLargestTextureDimension, gLargestTexturePixels,
						gLargestTexturePixels * bytesPerPixels,
						(int) srcTexWidth, (int) srcTexHeight );
					}
				}
			else
				{
				if (
					((int) gLargestTextureDimension < *dstTexWidth)
						||
					((int) gLargestTextureDimension < *dstTexHeight)
						||
					((int) gLargestTexturePixels < (*dstTexWidth) * (*dstTexHeight)) )
					{
					if ( (int) gLargestTextureDimension < *dstTexWidth )
						gLargestTextureDimension = *dstTexWidth;
					
					if ( (int) gLargestTextureDimension < *dstTexHeight )
						gLargestTextureDimension = *dstTexHeight;
					
					if ( (int) gLargestTexturePixels < (*dstTexWidth) * (*dstTexHeight) )
						gLargestTexturePixels = (*dstTexWidth) * (*dstTexHeight);
					
					DShowMessage( "Largest texture sizes: "
						"%d length, %d pixels (%d bytes) (%d w X %d h -> %d w X %d h)",
						gLargestTextureDimension, gLargestTexturePixels,
						gLargestTexturePixels * bytesPerPixels,
						(int) srcTexWidth, (int) srcTexHeight,
						(int) *dstTexWidth, (int) *dstTexHeight );
					}
				}
#endif	// DEBUG_VERSION
			}
		else
			{
#ifdef DEBUG_VERSION
			ShowMessage( "Could not load texture, releasing object..." );
#endif	// DEBUG_VERSION
			
			unbindTexture( textureObjectName );
			glDeleteTextures( 1, &textureObjectName );
				// also performs equivalent of glBindTexture(0)
			textureObjectName = 0;
			}
#ifdef DEBUG_VERSION
		}
	else
		{
		ShowMessage( "glGenTextures() failed" );
#endif	// DEBUG_VERSION
		}	// textureObjectName
	}
	
//	ShowOpenGLErrors();
	return returnVal;
}


void
createLightmap()
{
	if ( gUsingOpenGL )
		{
		if ( gOGLNightTexture )
			return;
		
			// we aren't going to worry about pathologic cases like width == 1 and such
			// we're also going to assume that any texture that failed to fit inside
			// GL_PROXY_TEXTURE_RECTANGLE_EXT would also fail to fit inside GL_PROXY_TEXTURE_2D
		
		GLint srcTexWidth = gLayout.layoFieldBox.rectRight - gLayout.layoFieldBox.rectLeft;
		GLint srcTexHeight = gLayout.layoFieldBox.rectBottom - gLayout.layoFieldBox.rectTop;
		
		GLint dstTexWidth = 1;
		while ( dstTexWidth < srcTexWidth )
			dstTexWidth = dstTexWidth << 1;
		
		GLint dstTexHeight = 1;
		while ( dstTexHeight < srcTexHeight )
			dstTexHeight = dstTexHeight << 1;
		
		glGenTextures( 1, &gOGLNightTexture );
		if ( gOGLNightTexture )
			{
			bindTexture2D( gOGLNightTexture );
				// this does NOT allocate memory,
				// it checks to see if we are within certain limits
#if 0
DShowMessage( "in: srcTexWidth, dstTsrcTexHeight:, %d x %d", srcTexWidth, srcTexHeight );
DShowMessage( "in: dstTexWidth, dstTexHeight:, %d x %d", dstTexWidth, dstTexHeight );
if ( dstTexWidth > srcTexWidth )
	dstTexWidth = dstTexWidth >> 1;
if ( dstTexHeight > srcTexHeight )
	dstTexHeight = dstTexHeight >> 1;
if ( dstTexWidth > dstTexHeight )
	gOGLNightTexSize = dstTexHeight;
else
	gOGLNightTexSize = dstTexWidth;
#endif  // 0

// was "test", now will probably keep it this way, since it looks just as good as the bigger ones
// 64 is special, because 64 is the biggest texture that the OGL standard guarantees will work
#ifdef OGL_UNSCALED_LIGHTMAP
			gOGLNightTexSize = 1024;
#else
			gOGLNightTexSize = 64;
#endif	// OGL_UNSCALED_LIGHTMAP
			
			gOGLNightTexHScale = GLfloat(gOGLNightTexSize) / GLfloat(srcTexWidth);
			gOGLNightTexVScale = GLfloat(gOGLNightTexSize) / GLfloat(srcTexHeight);
			
//	DShowMessage( "out: gOGLNightTexSize, gOGLNightTexHScale x gOGLNightTexVScale: %d, %f x %f",
//		gOGLNightTexSize, gOGLNightTexHScale, gOGLNightTexVScale );
			
			glTexImage2D(	GL_PROXY_TEXTURE_2D, 0, GL_RGBA,
							gOGLNightTexSize, gOGLNightTexSize, 0,
#ifdef OGL_UNSCALED_LIGHTMAP
							gOGLHaveBGRA ? GL_BGRA : GL_RGBA,
							gOGLHaveBGRA ? GL_UNSIGNED_INT_8_8_8_8_REV : GL_UNSIGNED_BYTE,
#else
							GL_RGBA, GL_UNSIGNED_INT,
								// for screen buffer grabs, what do I really want here?
#endif	// OGL_UNSCALED_LIGHTMAP
							nullptr );
			GLint testwidth;
			glGetTexLevelParameteriv( GL_PROXY_TEXTURE_2D, 0, GL_TEXTURE_WIDTH, &testwidth );
			if ( testwidth )
				{
					// this actually allocates memory, which might not be available
					
#ifdef OGL_SOFTWARE_CLIENT_STORAGE_LIGHTMAP
if(
	(
		( AGL_RENDERER_GENERIC_ID == gOGLRendererID )
			||
		( AGL_RENDERER_APPLE_SW_ID == gOGLRendererID )
	)
		&&
	(true)	// feature test for local storage, gOGLHaveBGRA && gOGLHavePackedPixels
)
	{
	}
else
	{
	}
#else
#endif	// OGL_SOFTWARE_CLIENT_STORAGE_LIGHTMAP
				
				glTexImage2D(	GL_TEXTURE_2D, 0,
#if 1
								GL_RGBA,
#else
								GL_RGB,
#endif
								gOGLNightTexSize, gOGLNightTexSize, 0,
#ifdef OGL_UNSCALED_LIGHTMAP
								gOGLHaveBGRA ? GL_BGRA : GL_RGBA,
								gOGLHaveBGRA ? GL_UNSIGNED_INT_8_8_8_8_REV : GL_UNSIGNED_BYTE,
#else
								GL_RGBA, GL_UNSIGNED_INT,
									// for screen buffer grabs, what do I really want here?
//								GL_BGRA, GL_UNSIGNED_INT_8_8_8_8_REV,
									// for screen buffer grabs, what do I really want here?
#endif	// OGL_UNSCALED_LIGHTMAP
								nullptr );

				if ( not ShowOpenGLErrors() )	// the most likely error is out-of-memory
					{
					if ( gOGLHaveClampToEdge )
						{
							// the default is GL_REPEAT, which isn't what we want,
							// but CL_CLAMP would require a border, and it isn't obvious
							// what the border color should be
						glTexParameteri( GL_TEXTURE_2D, GL_TEXTURE_WRAP_S, GL_CLAMP_TO_EDGE );
						glTexParameteri( GL_TEXTURE_2D, GL_TEXTURE_WRAP_T, GL_CLAMP_TO_EDGE );
						}
						// this will change (elsewhere) according to gFastBlendMode
						// GL_NEAREST is the faster (and preferred by OGL_UNSCALED_LIGHTMAP)
					glTexParameteri( GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_NEAREST );
					glTexParameteri( GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_NEAREST );
					gOGLNightTextureFilter = GL_NEAREST;
					}
				else
					{
					unbindTexture2D( gOGLNightTexture );
					glDeleteTextures( 1, &gOGLNightTexture );
						// also performs equivalent of glBindTexture(GL_TEXTURE_2D, 0)
					gOGLNightTexture = 0;
					gOGLNightTexturePriority = -1.0f;
					} // not ShowOpenGLErrors()
				}
			else
				{
				ShowMessage( "GL_PROXY_TEXTURE_2D failure" );
				unbindTexture2D( gOGLNightTexture );
				glDeleteTextures( 1, &gOGLNightTexture );
					// also performs equivalent of glBindTexture(GL_TEXTURE_2D, 0)
				gOGLNightTexture = 0;
				gOGLNightTexturePriority = -1.0f;
				}
			}
		}
	else
		{
		if ( gLightmapGWorld && gAllBlackGWorld )
			return;
		
#ifdef OGL_QUICKDRAW_LIGHT_EFFECTS
// note to self:  add "only as big as playfield" code (optimization)
		
		Rect box = { 0, 0, gLayout.layoWinHeight, gLayout.layoWinWidth };
		OSStatus err = ::NewGWorld( &gLightmapGWorld, 32, &box, 0, 0, 0 );
		if ( noErr != err )
			{
			ShowMessage( "NewGWorld failed with error %d", err );
			return;
			}
		else
			{
			err = ::NewGWorld( &gAllBlackGWorld, 32, &box, 0, 0, 0 );
			if ( noErr != err )
				{
				ShowMessage( "NewGWorld failed with error %d", err );
				::DisposeGWorld( gLightmapGWorld );
				gLightmapGWorld = nullptr;
				return;
				}
			else
				{
				CGrafPtr oldCGrafPtr;
				GDHandle oldGDHandle;
				::GetGWorld( &oldCGrafPtr, &oldGDHandle );
				
				::SetGWorld( gAllBlackGWorld, nullptr );
				PixMapHandle pixMap = ::GetGWorldPixMap( gAllBlackGWorld );
				::LockPixels( pixMap );
				::RGBBackColor( DTS2Mac(&DTSColor::black) );
				::EraseRect( DTS2Mac(&gLayout.layoFieldBox) );
				::RGBBackColor( DTS2Mac(&DTSColor::white) );
				::UnlockPixels( pixMap );
				
				::SetGWorld( oldCGrafPtr, oldGDHandle );
				}
			}
#endif	// OGL_QUICKDRAW_LIGHT_EFFECTS
		}
}


void
drawNightTextureObject()
{
	if ( not gOGLNightTexture || not gUseLightMap )
		return;	// don't do this if no light casters or no texture
	
	glColor3f( 1.0f, 1.0f, 1.0f );
	setTextureEnvironmentMode( GL_REPLACE );
	enableBlending();
	disableAlphaTest();
	glBlendFunc( GL_DST_COLOR, GL_ZERO );
		// this amounts to  dest * source; alpha doesn't matter
	if ( gOGLUseDither )
		glEnable( GL_DITHER );
	enableTexturing2D();
	
	bindTexture2D( gOGLNightTexture );
	
#ifndef OGL_UNSCALED_LIGHTMAP
		// linear filtering is free/cheap on any hardware, but nearly doubles
		// the software renderer's frametime.  unfortunately, the visual artifacts,
		// particularly where the lightmap has high gradients, may prove unacceptable.
		// We can reduce the artifacts by using a larger lightmap, at the cost of
		// taking even more time to render.  we'll have to wait for feedback from the users
	GLint desiredFilter = gFastBlendMode ? GL_NEAREST : GL_LINEAR;
	if ( desiredFilter != gOGLNightTextureFilter )
		{
		glTexParameteri( GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, desiredFilter );
		glTexParameteri( GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, desiredFilter );
		gOGLNightTextureFilter = desiredFilter;
		}
#endif	// OGL_UNSCALED_LIGHTMAP
	
	glBegin( GL_QUADS );
	
#ifdef OGL_UNSCALED_LIGHTMAP
		glTexCoord2f(
			0.0f,
			float( gLayout.layoFieldBox.rectBottom - gLayout.layoFieldBox.rectTop )
				/ float( gOGLNightTexSize ) );
#else
		glTexCoord2f( 0.0f, 1.0f );
#endif	// OGL_UNSCALED_LIGHTMAP
		
		glVertex2s( gLayout.layoFieldBox.rectLeft, gLayout.layoFieldBox.rectTop );
		
#ifdef OGL_UNSCALED_LIGHTMAP
		glTexCoord2f(
			float( gLayout.layoFieldBox.rectRight - gLayout.layoFieldBox.rectLeft )
				/ float( gOGLNightTexSize ),
			float( gLayout.layoFieldBox.rectBottom - gLayout.layoFieldBox.rectTop )
				/ float( gOGLNightTexSize ) );
#else
		glTexCoord2f( 1.0f, 1.0f );
#endif	// OGL_UNSCALED_LIGHTMAP
		
		glVertex2s( gLayout.layoFieldBox.rectRight, gLayout.layoFieldBox.rectTop );
		
#ifdef OGL_UNSCALED_LIGHTMAP
		glTexCoord2f(
			float( gLayout.layoFieldBox.rectRight - gLayout.layoFieldBox.rectLeft )
				/ float( gOGLNightTexSize ),
			0.0f);
#else
		glTexCoord2f( 1.0f, 0.0f );
#endif	// OGL_UNSCALED_LIGHTMAP
		
		glVertex2s( gLayout.layoFieldBox.rectRight, gLayout.layoFieldBox.rectBottom );
		
		glTexCoord2f( 0.0f, 0.0f );
		glVertex2s( gLayout.layoFieldBox.rectLeft, gLayout.layoFieldBox.rectBottom );
	glEnd();
	
	if ( gOGLUseDither )
		glDisable( GL_DITHER );
	
	glBlendFunc( GL_SRC_ALPHA, GL_ONE_MINUS_SRC_ALPHA );	// reset to what i had been using
}


/*
**	aglDeallocEntryPoints()
**	nothing to do
*/
void
aglDeallocEntryPoints()
{
}

