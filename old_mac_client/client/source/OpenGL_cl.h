/*
**	OpenGL_cl.h
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

#ifndef OPENGL_CL_H
#define OPENGL_CL_H

#ifndef USE_OPENGL
# define USE_OPENGL
#endif

// apple just has to be different
// on most any other platform (INCLUDING OS9!)
// this would  be <GL/gl.h>.
// it should never be <gl.h>, because
// that should point to the IRIX GL header

#include <OpenGL/gl.h>

	// if we get past this line, we have OGL 1.0
	// unfortunately, we need more than that (texture objects, among other things)
	// this isn't a big limitation (on the Mac), and I don't ever expect this to fail,
	// but I'm nothing if not paranoid
	//
	// warning:  if we do another platform, add a runtime 1.1 test to context selection

#ifndef GL_VERSION_1_1
# warning "This code depends on OpenGL version 1.1 or later (GL_VERSION_1_1)"
# undef USE_OPENGL
#endif	// ! GL_VERSION_1_1

#ifdef USE_OPENGL

#include <AGL/agl.h>
#include <OpenGL/glext.h>

// GL function pointer support
OSStatus	aglInitEntryPoints();
void		aglDeallocEntryPoints();

// feature-via-version tests
// extern bool gOGLHaveVersion_1_2;
// extern bool gOGLHaveVersion_1_3;
// extern bool gOGLHaveVersion_1_4;

// feature-via-extension tests
// to avoid cluttering up the code with lots of #if/#endif, we'll define things ourselves

// extern bool gOGLHaveTexRectExtension;
// core api:  none
// extension:  1.1 if GL_EXT_texture_rectangle or GL_NV_texture_rectangle
// note: OSX 10.2 uses these constants.  10.1 implements this extension with different constants.
// todo:  fix so we do the right thing under 10.1 (easiest to just disable)
// todo:  implement max texture size global, separate from the 2D global
#ifndef GL_TEXTURE_RECTANGLE_EXT
	#define GL_TEXTURE_RECTANGLE_EXT			0x84F5	// GL_TEXTURE_RECTANGLE_NV
#endif	// GL_TEXTURE_RECTANGLE_EXT
#ifndef GL_PROXY_TEXTURE_RECTANGLE_EXT
	#define GL_PROXY_TEXTURE_RECTANGLE_EXT		0x84F7	// GL_PROXY_TEXTURE_RECTANGLE_NV
#endif	// GL_PROXY_TEXTURE_RECTANGLE_EXT
#ifndef GL_MAX_RECTANGLE_TEXTURE_SIZE_EXT
	#define GL_MAX_RECTANGLE_TEXTURE_SIZE_EXT	0x84F8	// GL_MAX_RECTANGLE_TEXTURE_SIZE_NV
#endif	// GL_MAX_RECTANGLE_TEXTURE_SIZE_EXT

// extern bool gOGLHavePalettedTextureExtension;
// core api: none (likely never; feature has been abandoned.
//  we only care because it changes the behavior of texture_rectangle)
// extension:  GL_EXT_paletted_texture

// extern bool gOGLHaveClampToEdge;
// core api:  1.2
// extension:  GL_SGIS_texture_edge_clamp
#ifndef GL_CLAMP_TO_EDGE
	#ifndef GL_CLAMP_TO_EDGE_SGIS
		#define GL_CLAMP_TO_EDGE_SGIS 0x812F
	#endif	// GL_CLAMP_TO_EDGE_SGIS
	#define GL_CLAMP_TO_EDGE	GL_CLAMP_TO_EDGE_SGIS
#endif	// GL_CLAMP_TO_EDGE

// extern bool gOGLHaveBGRA;
// core api:  1.2
// extension:  GL_EXT_bgra
#ifndef GL_BGRA
	#ifndef GL_BGRA_EXT
		#define GL_BGRA_EXT 0x80E1
	#endif	// GL_BGRA_EXT
	#define GL_BGRA GL_BGRA_EXT
#endif	// GL_BGRA

// extern bool gOGLHavePackedPixels;
// core api:  1.2
// extension:  1.1 if GL_APPLE_packed_pixels.
// some early ATI drivers exported "GL_APPLE_packed_pixel" (no 's') by mistake, but we aren't
// going to worry about them because HaveExtension() should reject them automatically
// GL_EXT_packed_pixels is a predecessor but does NOT include GL_UNSIGNED_INT_8_8_8_8_REV
// so we aren't going to bother testing for it
#ifndef	GL_UNSIGNED_INT_8_8_8_8_REV
	#define GL_UNSIGNED_INT_8_8_8_8_REV 0x8367
#endif	// GL_UNSIGNED_INT_8_8_8_8_REV

extern bool gOGLHaveBlendSubtract;
// extern bool gOGLHaveBlendSubtractCoreFunction;	// glBlendEquation()
// extern bool gOGLHaveBlendSubtractExtension;		// glBlendEquationEXT()
// core api: 1.4, 1.2 if GL_ARB_imaging
// extension: 1.0 if GL_EXT_blend_subtract
//
// WARNING:  this causes trouble for 10.1.x and earlier, which doesn't automatically weaklink
// missing extensions, resulting in a failure to launch.  I've tried building with the entire
// GL library weaklinked, but I haven't heard back from any testers if this fixed the problem
//
// (Apparently not a problem anymore, for Mach-O builds)
#ifndef GL_FUNC_ADD_EXT
	#define GL_FUNC_ADD_EXT 0x8006
#endif	// GL_FUNC_ADD_EXT
#ifndef GL_FUNC_ADD
	#define GL_FUNC_ADD GL_FUNC_ADD_EXT
#endif	// GL_FUNC_ADD
#ifndef GL_FUNC_REVERSE_SUBTRACT_EXT
	#define GL_FUNC_REVERSE_SUBTRACT_EXT 0x800B
#endif	// GL_FUNC_REVERSE_SUBTRACT_EXT
#ifndef GL_FUNC_REVERSE_SUBTRACT
	#define GL_FUNC_REVERSE_SUBTRACT GL_FUNC_REVERSE_SUBTRACT_EXT
#endif	// GL_FUNC_REVERSE_SUBTRACT
// both glBlendEquation and glBlendEquationEXT use exactly the same semantics
typedef void (*glBlendEquationProcPtr)( GLenum mode );
extern glBlendEquationProcPtr gOGLglBlendEquation;	// gOGL... is getting a bit ungainly


// todo: integrate these better
extern GLuint gOGLTexTarget2D;	// gOGLNominalTextureTarget2D ?
// extern GLuint gOGLProxyTexTarget2D;	// gOGLNominalProxyTextureTarget2D ?

// texture rectange seems to be my bane
// fortunately, it may finally be fixed now
//	- turned off as per JR email, 1 Oct 2007
//  - on the other hand, turning it off seems to crash NVIDIA7300 MacPros, sez Torx
// #define OGL_USE_TEXTURE_RECTANGLE
// this was the solution.  if present, it changes the behavior of texture rectangle;
// unfortunately 10.3.4+ keeps the behavior, but forgets to export it to the extension string
// contrary to what one would assume, commenting this out will force us to assume the
// extension is present (which in our case is the "safe", though slower, path)
//
// update:  it should be safe to enable this now; we detect the render ID and act accordingly
// update 2: 10.3.6 broke it again.  sigh.
//#define OGL_DETECT_PALETTED_TEXTURE

// text fade experiment
#define OGL_USE_TEXT_FADE

// vertex array experiment
//
// normally, we'd prefer to use display lists for the kind of static "models"
// that CL uses, but for very tiny objects, and since the vertex data is
// largely precalculated, and because the overhead of the matrix calls reverses
// at this end of the scale, vertex arrays seem to make more sense than display
// lists do (something about it still bugs me, though)
//
// to do this, we need to assume that icBox and friends are "well behaved"
// (meaning they won't change except where we can easily track it), because we
// are going to cache the texture coordinates that are derived from them
#define OGL_USE_VERTEX_ARRAYS

// update enable/disable experiment
//#define OGL_USE_UPDATE_OVERRIDE

// unscaled lightmap experiment
//#define OGL_UNSCALED_LIGHTMAP

// seasons.  very preliminary
// ought not be a OGL_ thing, should work for QD version too
// eventually will need to determine season automatically,
// but for now i'll just hardcode something for proof-of-concept
// 0, 1, 2, 3 = winter, spring, summer, fall
//#define OGL_SEASONS 0
#ifdef OGL_SEASONS
	enum {kWinter, kSpring, kSummer, kFall};
#endif	// OGL_SEASONS

// displays drawtime in non-debug clients (for debugging non-debug client)
#ifdef DEBUG_VERSION
# define OGL_SHOW_DRAWTIME
#endif

// why am I crashing under 10.3.4 if this is enabled?
//#define OGL_USE_AGL_STATE_VALIDATION

//#define OGL_USE_AGL_SWAP_INTERVAL

// displays image id's (for finding problem images when using light casters)
//#define OGL_SHOW_IMAGE_ID

// temp:  once again, someone forgot the world of indexed color
#define BROKEN_TEXSUBIMAGE2D_COLOR_INDEX

// quickdraw light effects
// attached to the ogl stuff until I disentangle light effects from the ogl code
// ...which might not be so easy anymore...
//#define OGL_QUICKDRAW_LIGHT_EFFECTS
extern GWorldPtr gLightmapGWorld;
extern GWorldPtr gAllBlackGWorld;
extern int gLightmapRadiusLimit;

const int kLightmapScaleCircleArraySize = 256;
const int kLightmapMaximumRadius = kLightmapScaleCircleArraySize - 1;
	// also used to cap gLightmapRadiusLimit
	// this is about half the playfield, and
	// more than that doesn't really make much sense
	// since there's already enough weirdness with
	// lights popping on as mobiles enter the playfield
// extern int gCachedCircularLightmapElementRadius;

void qdLightmapCircularElementBlitter(
	Ptr baseAddr,
	int rowBytes,
	const Point& center,
	int radius,
	int minimumRadius,
	const RGBColor& colorus,
	bool lightdarkcaster
);
void qdClearLightmap( const RGBColor& moonlight, const RGBColor& starlight );
void qdApplyLightmap();
void qdClampLightmap( float nightCLampColor );

extern bool gOpenGLAvailable;	// determined at startup
extern bool gUsingOpenGL;		// are we currently using ogl

extern bool gOpenGLEnable;					// master enable; user pref
extern bool gOpenGLEnableEffects;			// ogl-specific effects, like smooth night; user pref
extern bool gOpenGLForceGenericRenderer;	// force the software renderer; user pref
extern bool gOpenGLPermitAnyRenderer;		// allow _old_ accelerators (Rage II, etc); user pref

	// some OGL state that we want to track globally
	// if there get to be too many of these, i'll make a struct or class
	// need to attach these to ctx, since these all are specific to a particular context
extern bool gOGLDisableShadowEffects;	// GL_MODULATE bug on RagePro/RageII
extern bool gOGLBlendingIsEnabled;
extern bool gOGLAlphaTestIsEnabled;
extern bool gOGLStencilBufferAvailable;
extern bool gOGLStencilTestIsEnabled;
extern GLuint gOGLCurrentEnabledTextureTarget;
	// GL_TEXTURE_2D or GL_TEXTURE_RECTANGLE_EXT; 0 = disabled
extern GLfloat gOGLTextureEnvironmentMode;
extern GLuint gOGLCurrentTexture;		// used for both GL_TEXTURE_2D and GL_TEXTURE_RECTANGLE_EXT
extern GLenum gOGLBlendEquationMode;
// extern bool gOGLPixelZoomIsEnabled;
extern GLuint gOGLSelectionList;	// maybe move to join nightList?
extern bool gOGLUseDither;

extern GLint gOGLNightTexSize;
extern GLfloat gOGLNightTexHScale, gOGLNightTexVScale;
extern GLuint gOGLNightTexture;		// change to gLightmapOGLTexture
extern GLclampf gOGLNightTexturePriority;
// extern GLint gOGLNightTextureFilter;
extern GLuint gLightList;
extern int gNumLightCasters;
extern int gNumDarkCasters;
extern bool gUseLightMap;

void createLightmap();
void fillNightTextureObject();
void drawNightTextureObject();
void countLightDarkCasters();

#ifdef OGL_USE_VERTEX_ARRAYS
extern const GLvoid * gOGLVertexArray;
extern const GLvoid * gOGLTexCoordArray;
#endif	// OGL_USE_VERTEX_ARRAYS

#ifdef DEBUG_VERSION
	// informational; displayed in message window
extern uint gLargestTextureDimension;
extern uint gLargestTexturePixels;
#endif

#ifdef OGL_QUICKDRAW_LIGHT_EFFECTS
// uh...
#endif	// OGL_QUICKDRAW_LIGHT_EFFECTS

extern const char * gOGLRendererString;
#ifdef OGL_SHOW_DRAWTIME
extern GLint gOGLRendererID;
#endif	// OGL_SHOW_DRAWTIME
//extern bool gOGLRendererRage128;
//extern bool gOGLRendererGeneric;
extern bool gOGLManageTexturePriority;
//extern bool gOGLRendererSmallTextureMemory;

	// this needs a better name...
	// no, i need to make a proper struct (or obj) out of it
extern AGLContext ctx;

enum IndexToAlphaMapID
{
	kIndexToAlphaMapAlpha100TransparentZero=0,
	kIndexToAlphaMapAlpha25,
	kIndexToAlphaMapAlpha25TransparentZero,
	kIndexToAlphaMapAlpha50,
	kIndexToAlphaMapAlpha50TransparentZero,
	kIndexToAlphaMapAlpha75,
	kIndexToAlphaMapAlpha75TransparentZero,
	kIndexToAlphaMapCount,
//	kIndexToAlphaMapAlpha100	// doesn't need an actual map, cheaper to turn off blending, etc
	kIndexToAlphaMapNone		// easier to understand
};

extern GLint gPixelMapSize;
// i didn't used to save rgb maps, but the broken index to color bug
// in texsubimage makes this necessary
extern GLushort *	gIndexToRedMap;
extern GLushort *	gIndexToGreenMap;
extern GLushort *	gIndexToBlueMap;
typedef GLushort IndexToAlphaMapArray[ 256 ];

extern IndexToAlphaMapArray IndexToAlphaMap[ kIndexToAlphaMapCount ];
extern IndexToAlphaMapID gOGLCurrentIndexToAlphaMap;

bool isOpenGLAvailable();
bool HaveExtension( const char * extName );

	// need to supply __FILE__ and __LINE__ at point of use,
	// but default parameters always point here
	// macros sidestep that
#define ShowAGLErrors() DisplayAGLErrors( __FILE__, __LINE__ )
bool DisplayAGLErrors( const char * file, int line );

#define ShowOpenGLErrors() DisplayOpenGLErrors( __FILE__, __LINE__ )
bool DisplayOpenGLErrors( const char * file, int line );

#if 0
class OpenGLContext
{
	public:
		OpenGLContext() :
			context(nullptr)
		{};
		virtual ~OpenGLContext()
			{
				// last of all...
			if ( context )
				{
				if ( ! aglDestroyContext( context ) )
					ShowAGLErrors();
				
				context = nullptr;
				}
			}
		
//		inline setOpenGLContext( AGLContext contextIn ) {}

		inline void enableBlending();
		inline void enableAlphaTest();
		inline void enableTexturing();
		inline void disableBlending();
		inline void disableAlphaTest();
		inline void disableTexturing();
	protected:
	private:
			// make these private, so they can't be called
			// since we have pointer issues, we need to write
			// explicit constructors if we ever use these
		OpenGLContext( const OpenGLContext &original );			// copy contructor
		OpenGLContext& operator=( const OpenGLContext &rhs );	// assignment

		AGLContext context;
};
#else

inline void
enableBlending()
{
	if ( ! gOGLBlendingIsEnabled )
		{
		glEnable( GL_BLEND );
		gOGLBlendingIsEnabled = true;
		}
}


inline void
disableBlending()
{
	if ( gOGLBlendingIsEnabled )
		{
		glDisable( GL_BLEND );
		gOGLBlendingIsEnabled = false;
		}
}


inline void
enableAlphaTest()
{
	if ( ! gOGLAlphaTestIsEnabled )
		{
		glEnable( GL_ALPHA_TEST );
		gOGLAlphaTestIsEnabled = true;
		}
}


inline void
disableAlphaTest()
{
	if ( gOGLAlphaTestIsEnabled )
		{
		glDisable( GL_ALPHA_TEST );
		gOGLAlphaTestIsEnabled = false;
		}
}


inline void
enableStencilTest()
{
	if ( gOGLStencilBufferAvailable && ! gOGLStencilTestIsEnabled )
		{
		glEnable( GL_STENCIL_TEST );
		gOGLStencilTestIsEnabled = true;
		}
}


inline void
disableStencilTest()
{
	if ( gOGLStencilBufferAvailable && gOGLStencilTestIsEnabled )
		{
		glDisable( GL_STENCIL_TEST );
		gOGLStencilTestIsEnabled = false;
		}
}


inline void
enableTexturing2D()
{
	if ( gOGLCurrentEnabledTextureTarget != GL_TEXTURE_2D )
		{
		if ( gOGLCurrentEnabledTextureTarget )	// something else is already enabled
			glDisable( gOGLCurrentEnabledTextureTarget );
		
		gOGLCurrentEnabledTextureTarget = GL_TEXTURE_2D;
		glEnable( gOGLCurrentEnabledTextureTarget );
		}
}


inline void
enableTexturing()
{
	if ( gOGLCurrentEnabledTextureTarget != gOGLTexTarget2D )
		{
		if ( gOGLCurrentEnabledTextureTarget )	// something else is already enabled
			glDisable( gOGLCurrentEnabledTextureTarget );
		
		gOGLCurrentEnabledTextureTarget = gOGLTexTarget2D;
		glEnable( gOGLCurrentEnabledTextureTarget );
		}
}


inline void
disableTexturing()
{
	if ( gOGLCurrentEnabledTextureTarget )
		{
		glDisable( gOGLCurrentEnabledTextureTarget );
		gOGLCurrentEnabledTextureTarget = 0;
		}
}


inline void
setTextureEnvironmentMode( GLfloat newMode )
{
	if ( newMode != gOGLTextureEnvironmentMode )
		{
		glTexEnvf( GL_TEXTURE_ENV, GL_TEXTURE_ENV_MODE, newMode );
		gOGLTextureEnvironmentMode = newMode;
		}
}


inline void
setIndexToAlphaMap( IndexToAlphaMapID alphamap )
{
	if ( alphamap != gOGLCurrentIndexToAlphaMap )
		{
		glPixelMapusv( GL_PIXEL_MAP_I_TO_A, gPixelMapSize, IndexToAlphaMap[alphamap] );
		gOGLCurrentIndexToAlphaMap = alphamap;
		}
}


	// these next 4 could be better
inline void
bindTexture2D( GLuint texture )
{
	if ( texture != gOGLCurrentTexture )
		{
		glBindTexture( GL_TEXTURE_2D, texture );
		gOGLCurrentTexture = texture;
		}
}


inline void
unbindTexture2D( GLuint texture )
{
	if ( texture )
		{
		if ( texture == gOGLCurrentTexture )
			{
			glBindTexture( GL_TEXTURE_2D, 0 );
			gOGLCurrentTexture = 0;
			}
		}
}


inline void
bindTexture( GLuint texture )
{
	if ( texture != gOGLCurrentTexture )
		{
		glBindTexture( gOGLTexTarget2D, texture );
		gOGLCurrentTexture = texture;
		}
}


inline void
unbindTexture( GLuint texture )
{
	if ( texture
	&&   texture == gOGLCurrentTexture )
		{
		glBindTexture( gOGLTexTarget2D, 0 );
		gOGLCurrentTexture = 0;
		}
}


inline void
setBlendEquation( GLenum newMode = GL_FUNC_ADD )
{
			// how far do we need to go to accommodate old build environments?
	if ( newMode != gOGLBlendEquationMode )
		{
		gOGLBlendEquationMode = newMode;
		if ( gOGLHaveBlendSubtract )
			gOGLglBlendEquation( newMode );
		else
			{
			// if neither gOGLHaveBlendSubtractCoreFunction nor gOGLHaveBlendSubtractExtension,
			// then the blend equation can't be changed; we should never have been called.
			// patch up and return
			gOGLBlendEquationMode = GL_FUNC_ADD;
			}
		}
}


#ifdef OGL_USE_VERTEX_ARRAYS
inline void
setVertexArray( GLint size, GLenum arrayType, GLsizei stride, const GLvoid * pointer )
{
	if ( pointer != gOGLVertexArray )
		{
		glVertexPointer( size, arrayType, stride, pointer );
		gOGLVertexArray = pointer;
		}
}


inline void
setTexCoordArray( GLint size, GLenum arrayType, GLsizei stride, const GLvoid * pointer )
{
	if ( pointer != gOGLTexCoordArray )
		{
		glTexCoordPointer( size, arrayType, stride, pointer );
		gOGLTexCoordArray = pointer;
		}
}
#endif	// OGL_USE_VERTEX_ARRAYS

#endif	// ! 0


void setOGLState();

struct PixelFormatAttributes
{
	bool	useRGBColorSize;
	GLint	rgbColorSize;
	bool	useStencilSize;
	GLint	stencilSize;
	bool	useRendererID;
	GLint	rendererID;
	bool	permitAnyRenderer;
	bool	useNoRecovery;
	bool	useAccelerated;
};

const int kOGLMaxAttribArraySize = 16;	// need a more automatic way to maintain this

void attachOGLContextToGameWindow();
#if 0
void initOGLIndexToColorTables( GLushort * i2r, GLushort * i2g, GLushort * i2b, GLint mapsize );
#endif
void initOpenGLGameWindowFonts( AGLContext ctx );
void deleteOpenGLGameWindowObjects();
void deleteNonOpenGLGameWindowObjects();
void setupOpenGLGameWindow( AGLContext ctx, bool visible );
void DeleteTextureObjects();
void DecrementTextureObjectPriorities();
bool deleteAllUnusedTextures();
//bool deleteFirstUnusedTextures();
void drawOGLText( GLint x, GLint y, GLuint fontListBase, const char * text );
void drawOGLText( GLint x, GLint y, GLuint fontListBase, const char * text, GLsizei length );
void drawOGLPixmap(	const DTSRect& source,
					const DTSRect& destination,
					GLuint rowLength,
					IndexToAlphaMapID alphamap,
					void * pixels );


class TextureObject	// this is an abstract virtual base class
{
public:
		enum TextureObjectType
			{
			kNoTexture = 0,
			kSingleTile,
			kNAnimatedTiles,
			kMobileArray,
			kBalloonArray
			};
		
		TextureObject()
#ifdef OGL_SHOW_DRAWTIME
			 : textureCount(0)
#endif	// OGL_SHOW_DRAWTIME
			{}
		virtual ~TextureObject() {}
		
		virtual void decrementTextureObjectPriority() = 0;
		virtual bool deleteUnusedTextures() = 0;
		
		virtual void draw(	const DTSRect& src,
							const DTSRect& dst,
							GLuint rowLength,
							IndexToAlphaMapID alphamap,
							GLfloat targetAlpha,
							ImageCache * cache ) = 0;
		
		virtual bool drawAsDropShadow(	const DTSRect& src,
										const DTSRect& dst,
										int hOffset,
										int vOffset,
										float shadowAlpha,
										ImageCache * cache ) = 0;
		
		virtual bool drawAsRotatedShadow(	const DTSRect& src,
											const DTSRect& dst,
											float angle,
											float scale,
											float shadowAlpha,
											ImageCache * cache ) = 0;
		
#ifdef OGL_SHOW_DRAWTIME
		inline int getTextureCount() const { return textureCount; }
#endif	// OGL_SHOW_DRAWTIME
	
protected:
		virtual void load(	const DTSRect& src,
							IndexToAlphaMapID alphamap,
							ImageCache * cache ) = 0;
		bool loadTextureObject(	GLuint & textureObjectName,
							GLint hOffset,
							GLint vOffset,
							GLint srcTexWidth,
							GLint srcTexHeight,
							GLint * dstTexWidth,
							GLint * dstTexHeight,
							GLint clamp,
							ImageCache * cache,
							IndexToAlphaMapID alphamap );
		
#ifdef OGL_SHOW_DRAWTIME
		int textureCount;
#endif	// OGL_SHOW_DRAWTIME
};


void
drawOGLPixmapAsTexture(
	const DTSRect& src,
	const DTSRect& dst,
	GLuint rowLength,
	IndexToAlphaMapID alphamap,
	TextureObject::TextureObjectType textureObjectType,
	ImageCache * cache,
	GLfloat targetAlpha = 1.0f );


class TextureObjectSingle : public TextureObject
{
private:
			// make these private, so they can't be called
			// since we have pointer members, we need to write
			// explicit constructors if we ever use these
		TextureObjectSingle( const TextureObjectSingle &original );	// copy contructor
		TextureObjectSingle& operator=( const TextureObjectSingle &rhs );	// assignment

public:
		TextureObjectSingle() :
			TextureObject(),
			name(0),
			priority(1.0f) {}
		
		virtual ~TextureObjectSingle()
			{
			if ( name )
				{
#ifdef DEBUG_VERSION
				if ( GL_FALSE == glIsTexture( name ) )
					ShowMessage( "Error:  ~TextureObjectSingle(): "
								  "invalid texture object in TextureObjectSingle" );
#endif	// DEBUG_VERSION
				
				unbindTexture( name );
				glDeleteTextures( 1, &name );
				}
			name = 0;
			}
		
		GLuint		name;			// this is the OpenGL 'name' for this object
		GLclampf	priority;		// this is the OpenGL priority for this object
		GLfloat		invTexWidth;	// actual (power of 2) size inverse
		GLfloat		invTexHeight;	// actual (power of 2) size inverse
		GLfloat		texCoord[8];	// 4 vertex * 2 coords (s,t) at each vertex
									// close packed (single dimension) cuz GL likes it that way
		
		virtual void decrementTextureObjectPriority()
			{
			if ( name && gOGLManageTexturePriority )
				{
#ifdef DEBUG_VERSION
				if ( GL_FALSE == glIsTexture( name ) )
					{
					ShowMessage( "Error:  invalid texture object in TextureObjectSingle" );
					return;
					}
#endif	// DEBUG_VERSION
				
				priority -= 0.5f;
				if ( priority < 0.0f )
					priority = 0.0f;
				glPrioritizeTextures( 1, &name, &priority );
				}
			}
		
		virtual bool deleteUnusedTextures()
			{
			bool result = false;
			if ( name && gOGLManageTexturePriority )
				{
#ifdef DEBUG_VERSION
				if ( GL_FALSE == glIsTexture( name ) )
					{
					ShowMessage( "Error:  invalid texture object in TextureObjectSingle" );
					return false;
					}
#endif	// DEBUG_VERSION
				
				if ( priority <= 0.0f )	// never use == with a float
					{
//					ShowMessage( "TextureObjectSingle priority == 0, deleting..." );
					unbindTexture( name );
					glDeleteTextures( 1, &name );
					
#ifdef OGL_SHOW_DRAWTIME
					--textureCount;
#endif	// OGL_SHOW_DRAWTIME
					
					name = 0;
					result = true;
					}
				}
			return result;
			}
		
		virtual void draw(	const DTSRect& src,
							const DTSRect& dst,
							GLuint rowLength,
							IndexToAlphaMapID alphamap,
							GLfloat /*targetAlpha*/,
							ImageCache * cache );
		
		virtual bool drawAsDropShadow(	const DTSRect& src,
										const DTSRect& dst,
										int hOffset,
										int vOffset,
										float shadowAlpha,
										ImageCache * cache );
		
		virtual bool drawAsRotatedShadow(	const DTSRect& src,
											const DTSRect& dst,
											float angle,
											float scale,
											float shadowAlpha,
											ImageCache * cache );
protected:
		virtual void load(	const DTSRect& src,
							IndexToAlphaMapID alphamap,
							ImageCache * cache );
};


class TextureObjectNAnimated : public TextureObject
{
private:
			// make these private, so they can't be called
			// since we have pointer members, we need to write
			// explicit constructors if we ever use these
		TextureObjectNAnimated( const TextureObjectNAnimated &original );	// copy contructor
		TextureObjectNAnimated& operator=( const TextureObjectNAnimated &rhs );	// assignment

public:
		TextureObjectNAnimated() :
			TextureObject(),
			name(nullptr),
			priority(nullptr),
			totalFrames(0) {}
		
		virtual ~TextureObjectNAnimated()
			{
			if ( name )
				{
				for ( uint i = 0; i < totalFrames; ++i )
					{
#ifdef DEBUG_VERSION
					if ( name[i] )
						{
						if ( GL_FALSE == glIsTexture( name[i] ) )
							ShowMessage(
								"Error: invalid texture object in TextureObjectNAnimated" );
						}
#endif	// DEBUG_VERSION
					
					unbindTexture( name[i] );
					}
				
				glDeleteTextures( totalFrames, name );
				delete[] name;
				name = nullptr;
				delete[] priority;
				priority = nullptr;
				}
			}
		
		GLuint *	name;			// this is the OpenGL 'name' for this object
		GLclampf *	priority;		// this is the OpenGL priority for this object
									// note that we have set up the code so that priority is
									// valid only if name is, so we only need check name
		GLuint		totalFrames;
		GLfloat		invTexWidth;	// actual (power of 2) size inverse
		GLfloat		invTexHeight;	// actual (power of 2) size inverse
		GLfloat		texCoord[8];	// 4 vertex * 2 coords (s,t) at each vertex
									// close packed (single dimension) cuz GL likes it that way
									// NOTE THAT THIS IS NOT AN ARRAY POINTER
		
		virtual void decrementTextureObjectPriority()
			{
			if ( name && gOGLManageTexturePriority )
				{
				for ( uint i = 0; i < totalFrames; ++i )
					{
#ifdef DEBUG_VERSION
					if ( name[i] )
						{
						if ( GL_FALSE == glIsTexture( name[i] ) )
							ShowMessage(
								"Error: invalid texture object in TextureObjectNAnimated" );
						}
#endif	// DEBUG_VERSION
					
					if ( totalFrames )
						priority[i] -= 1.0f / totalFrames;
					else
						priority[i] -= 0.5f;
					
					if ( priority[i] < 0.0f )
						priority[i] = 0.0f;
					}
				glPrioritizeTextures( totalFrames, name, priority );
				}
			}
		
		virtual bool deleteUnusedTextures()
			{
			bool result = false;
			if ( name && gOGLManageTexturePriority )
				{
				for ( uint i = 0; i < totalFrames; ++i )
					{
#ifdef DEBUG_VERSION
					if ( name[i] )
						{
						if ( GL_FALSE == glIsTexture( name[i] ) )
							ShowMessage(
								"Error: invalid texture object in TextureObjectNAnimated" );
						}
#endif	// DEBUG_VERSION
					
					if ( name[i] && priority[i] <= 0.0f )	// never use == with a float
						{
						ShowMessage( "TextureObjectNAnimated priority == 0, deleting..." );
						unbindTexture( name[i] );
						glDeleteTextures( 1, &name[i] );
						
#ifdef OGL_SHOW_DRAWTIME
						--textureCount;
#endif	// OGL_SHOW_DRAWTIME
						
						name[i] = 0;
						result = true;
						}
					}
				}
			return result;
			}
		
		virtual void draw(	const DTSRect& src,
							const DTSRect& dst,
							GLuint rowLength,
							IndexToAlphaMapID alphamap,
							GLfloat /*targetAlpha*/,
							ImageCache * cache );
		
		virtual bool drawAsDropShadow(	const DTSRect& src,
										const DTSRect& dst,
										int hOffset,
										int vOffset,
										float shadowAlpha,
										ImageCache * cache );
		virtual bool drawAsRotatedShadow(	const DTSRect& src,
											const DTSRect& dst,
											float angle,
											float scale,
											float shadowAlpha,
											ImageCache * cache );
		
protected:
		virtual void load(	const DTSRect& src,
							IndexToAlphaMapID alphamap,
							ImageCache * cache );
};


class TextureObjectMobileArray : public TextureObject
{
private:
			// make these private, so they can't be called
			// since we have pointer members, we need to write
			// explicit constructors if we ever use these
		TextureObjectMobileArray( const TextureObjectMobileArray &original );	// copy ctor
		TextureObjectMobileArray& operator=( const TextureObjectMobileArray &rhs );	// assignment

public:
		enum {
			kWidth = 16,
			kHeight = 3,
			kArraySize = kWidth * kHeight
			};
		TextureObjectMobileArray() :
			TextureObject()
			{
			for ( int w = 0; w < kWidth; ++w )
				{
				for ( int h = 0; h < kHeight; ++h )
					{
					name[w][h] = 0;
					priority[w][h] = 1.0f;
					}
				}
			}
		
		virtual ~TextureObjectMobileArray()
			{
			for ( int w = 0; w < kWidth; ++w )
				{
				for ( int h = 0; h < kHeight; ++h )
					{
#ifdef DEBUG_VERSION
					if ( name[w][h] )
						{
						if ( GL_FALSE == glIsTexture( name[w][h] ) )
							ShowMessage( "Error: ~TextureObjectMobileArray(): "
								"invalid texture object in TextureObjectMobileArray" );
						}
#endif	// DEBUG_VERSION
					
					unbindTexture( name[w][h] );
					}
				}
			
			glDeleteTextures( kArraySize, &name[0][0] );
			for ( int w = 0; w < kWidth; ++w )
				{
				for ( int h = 0; h < kHeight; ++h )
					name[w][h] = 0;
				}
			}
		
		GLuint		name[kWidth][kHeight];		// this is the OpenGL 'name' for this object
		GLclampf	priority[kWidth][kHeight];	// this is the OpenGL priority for this object
		GLfloat		invTexSize;					// actual (power of 2) size inverse
		GLfloat		texCoord[8];				// 4 vertex * 2 coords (s,t) at each vertex
									// close packed (single dimension) cuz GL likes it that way
									// NOTE THAT THIS IS NOT AN ARRAY POINTER
		
		virtual void decrementTextureObjectPriority()
			{
			if ( gOGLManageTexturePriority )
				{
				for ( int w = 0; w < kWidth; ++w )
					{
					for ( int h = 0; h < kHeight; ++h )
						{
#ifdef DEBUG_VERSION
						if ( name[w][h] )
							{
							if ( GL_FALSE == glIsTexture( name[w][h] ) )
								ShowMessage(
									"Error:  invalid texture object in TextureObjectMobileArray" );
							}
#endif	// DEBUG_VERSION
						
						priority[w][h] -= 0.2f;	// walking: left stand right stand 
						if ( priority[w][h] < 0.0f )
							priority[w][h] = 0.0f;
						}
					}
				glPrioritizeTextures( kArraySize, &name[0][0], &priority[0][0] );
				}
			}
		
		virtual bool deleteUnusedTextures()
			{
			bool result = false;
			if ( gOGLManageTexturePriority )
				{
				for ( int w = 0; w < kWidth; ++w )
					{
					for ( int h = 0; h < kHeight; ++h )
						{
#ifdef DEBUG_VERSION
						if ( name[w][h] )
							{
							if ( GL_FALSE == glIsTexture( name[w][h] ) )
								ShowMessage( "Error: "
									"invalid texture object in TextureObjectMobileArray" );
							}
#endif	// DEBUG_VERSION
						
						if ( name[w][h] && priority[w][h] <= 0.0f )	// never use == with a float
							{
							ShowMessage( "TextureObjectMobileArray priority == 0, deleting..." );
							unbindTexture( name[w][h] );
							glDeleteTextures( 1, &name[w][h] );
							
#ifdef OGL_SHOW_DRAWTIME
							--textureCount;
#endif	// OGL_SHOW_DRAWTIME
							
							name[w][h] = 0;
							result = true;
							}
						}
					}
				}
			return result;
			}
		
		virtual void draw(	const DTSRect& src,
							const DTSRect& dst,
							GLuint rowLength,
							IndexToAlphaMapID alphamap,
							GLfloat /*targetAlpha*/,
							ImageCache * cache );
		
		virtual bool drawAsDropShadow(	const DTSRect& src,
										const DTSRect& dst,
										int hOffset,
										int vOffset,
										float shadowAlpha,
										ImageCache * cache );
		
		virtual bool drawAsRotatedShadow(	const DTSRect& src,
											const DTSRect& dst,
											float angle,
											float scale,
											float shadowAlpha,
											ImageCache * cache );
		
protected:
		virtual void load(	const DTSRect& src,
							IndexToAlphaMapID alphamap,
							ImageCache * cache );
};


class TextureObjectBalloonArray : public TextureObject
{
private:
			// make these private, so they can't be called
			// since we have pointer members, we need to write
			// explicit constructors if we ever use these
		TextureObjectBalloonArray( const TextureObjectBalloonArray &original );	// copy ctor
		TextureObjectBalloonArray& operator=( const TextureObjectBalloonArray &rhs );	// assign

public:
		enum {
			kWidth = 6,
			kHeight = 4,
			kArraySize = kWidth * kHeight
			};
		
		TextureObjectBalloonArray() :
			TextureObject()
			{
			for ( int w = 0; w < kWidth; ++w )
				{
				for ( int h = 0; h < kHeight; ++h )
					{
					name[w][h] = 0;
					priority[w][h] = 1.0f;
					}
				}
			}
		
		virtual ~TextureObjectBalloonArray()
			{
//			ShowMessage( "~TextureObjectBalloonArray()" );
			for ( int w = 0; w < kWidth; ++w )
				{
				for ( int h = 0; h < kHeight; ++h )
					{
#ifdef DEBUG_VERSION
					if ( name[w][h] )
						{
						if ( GL_FALSE == glIsTexture( name[w][h] ) )
							ShowMessage( "Error: "
								"invalid texture object in TextureObjectBalloonArray" );
						}
#endif	// DEBUG_VERSION
					
					unbindTexture( name[w][h] );
					}
				}
			glDeleteTextures( kArraySize, &name[0][0] );
			for ( int w = 0; w < kWidth; ++w )
				{
				for ( int h = 0; h < kHeight; ++h )
					name[w][h] = 0;
				}
			}
		
		GLuint		name[kWidth][kHeight];		// this is the OpenGL 'name' for this object
		GLclampf	priority[kWidth][kHeight];	// this is the OpenGL priority for this object
		GLfloat		invTexWidth;				// actual (power of 2) size inverse
		GLfloat		invTexHeight;				// actual (power of 2) size inverse
		GLfloat		texCoord[8];				// 4 vertex * 2 coords (s,t) at each vertex
										// close packed (single dimension) cuz GL likes it that way
										// NOTE THAT THIS IS NOT AN ARRAY POINTER
		
		virtual void decrementTextureObjectPriority()
			{
			if ( gOGLManageTexturePriority )
				{
				for ( int w = 0; w < kWidth; ++w )
					{
					for ( int h = 0; h < kHeight; ++h )
						{
#ifdef DEBUG_VERSION
						if ( name[w][h] )
							{
							if ( GL_FALSE == glIsTexture( name[w][h] ) )
								ShowMessage( "Error: "
									"invalid texture object in TextureObjectBalloonArray" );
							}
#endif	// DEBUG_VERSION
						
						priority[w][h] -= 0.5f;
						if ( priority[w][h] < 0.0f )
							priority[w][h] = 0.0f;
						}
					}
				glPrioritizeTextures( kArraySize, &name[0][0], &priority[0][0] );
				}
			}
		
		virtual bool deleteUnusedTextures()
			{
			bool result = false;
			if ( gOGLManageTexturePriority )
				{
				for ( int w = 0; w < kWidth; ++w )
					{
					for ( int h = 0; h < kHeight; ++h )
						{
#ifdef DEBUG_VERSION
						if ( name[w][h] )
							{
							if ( GL_FALSE == glIsTexture( name[w][h] ) )
								ShowMessage( "Error: "
									"invalid texture object in TextureObjectBalloonArray" );
							}
#endif	// DEBUG_VERSION
						
						if ( name[w][h] && priority[w][h] <= 0.0f )	// never use == with a float
							{
							ShowMessage( "TextureObjectBalloonArray priority == 0, deleting..." );
							unbindTexture( name[w][h] );
							glDeleteTextures( 1, &name[w][h] );
							
#ifdef OGL_SHOW_DRAWTIME
							--textureCount;
#endif	// OGL_SHOW_DRAWTIME
							
							name[w][h] = 0;
							result = true;
							}
						}
					}
				}
			return result;
			}
		
		virtual void draw(	const DTSRect& src,
							const DTSRect& dst,
							GLuint rowLength,
							IndexToAlphaMapID alphamap,
							GLfloat targetAlpha,
							ImageCache * cache );
		
		virtual bool drawAsDropShadow(	const DTSRect& src,
										const DTSRect& dst,
										int hOffset,
										int vOffset,
										float shadowAlpha,
										ImageCache * cache );
		
		virtual bool drawAsRotatedShadow(	const DTSRect& src,
											const DTSRect& dst,
											float angle,
											float scale,
											float shadowAlpha,
											ImageCache * cache );
		
protected:
		virtual void load(	const DTSRect& src,
							IndexToAlphaMapID alphamap,
							ImageCache * cache );
};

#endif	//  USE_OPENGL

#endif	// OPENGL_CL_H

