/*
**	Local_dts.h		dtslib2
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

#ifndef Local_dts_h
#define Local_dts_h

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif


// possible brain damage
#ifndef DTS_BIG_ENDIAN
  // maybe the compiler knows?
  
  // GCC or some system header might set these...
#  if defined( __BIG_ENDIAN__ ) || defined( _BIG_ENDIAN )|| \
  ( defined( __BYTE_ORDER ) && defined( __BIG_ENDIAN ) && ( __BYTE_ORDER == __BIG_ENDIAN ) )
#	define DTS_BIG_ENDIAN		1
#	define DTS_LITTLE_ENDIAN	0
#  elif defined ( __LITTLE_ENDIAN__ ) || defined( _LITTLE_ENDIAN )|| \
  ( defined( __BYTE_ORDER ) && defined( __LITTLE_ENDIAN ) && ( __BYTE_ORDER == __LITTLE_ENDIAN ) )
#	define DTS_BIG_ENDIAN		0
#	define DTS_LITTLE_ENDIAN	1

#  else
  	// we have no idea. Let's assume big-endian.
#  	warning "Unknown endianness for architecture."
#	define DTS_BIG_ENDIAN		1
#	define DTS_LITTLE_ENDIAN	0
#  endif

#endif	// ! defined( DTS_BIG_ENDIAN )


/*
**	Support for various compilers (GCC and Clang for now)
*/
#if defined( __GNUC__ )
	// any reasonably modern GCC can do these
# define PRINTF_LIKE( p_arg, d_arg ) __attribute__(( __format__( printf, (p_arg), (d_arg) )))
# define SCANF_LIKE( p_arg, d_arg )	 __attribute__(( __format__( scanf,  (p_arg), (d_arg) )))
# define DOES_NOT_RETURN			 __attribute__(( __noreturn__ ))
# define NOT_USED					 __attribute__(( __unused__ ))
#endif	// __GNUC__

// Support for control of symbol visibility
#if defined( __GNUC__ ) && ( __GNUC__ >= 4 )
# define EXPORTED __attribute__(( visibility("default") ))
#else
	// must use ".exp" files, or other mechanisms
# define EXPORTED
#endif

#ifndef __has_attribute
# define __has_attribute( x )	(0)
#endif

// this can suppress overzealous -Wimplicit-fallthrough warnings from GCC 7+
// (but clang 4.2 from Xcode 4.6.3 lies about supporting the attribute;
// dunno which version actually began, you know, honoring it.)
#if __has_attribute( __fallthrough__ ) && ( (__clang_major__ > 4) || (__GNUC__ > 4) )
#  define FALLTHRU_OK	__attribute__(( __fallthrough__ ))
#else
# define FALLTHRU_OK	/* empty */
#endif

// deal with c++11-isms related to exceptions
#if __cplusplus < 201103L
# define DOES_NOT_THROW			throw()
# define THROWS_BAD_ALLOC		throw( std::bad_alloc )
#else
// these throw()s are deprecated, apparently
# define DOES_NOT_THROW			noexcept
# define THROWS_BAD_ALLOC		/* empty */
#endif

// now we can use 'nullptr' even in c++98/03 mode
#if __cplusplus < 201103L
# ifndef nullptr
#  define nullptr				NULL
# endif
#endif  // c++11


/*
**	Define shorthand for unsigned integers
*/
typedef unsigned char		uchar;
typedef unsigned short		ushort;
typedef unsigned int		uint;
typedef unsigned long		ulong;


/*
**	Define needed things that are not already defined
*/
#ifndef __MACTYPES__
# define noErr			0
typedef uchar	DTSBoolean;
#else
typedef Boolean	DTSBoolean;
#endif	// __MACTYPES__

#ifndef __MACERRORS__
const int	memFullErr		= -108;
#endif

#define MakeLong( H, L )		( ((H) << 16) | ((L) & 0x0FFFF) )


/*
**	Define Error handling.
*/
typedef int		DTSError;
extern DTSError	gErrorCode;				// set whenever anything goes wrong
extern bool		bDebugMessages;			// set true if want debug messages when things go bad

void			SetErrorCode( DTSError result );	// routine to set the error code
inline void		ClearErrorCode()					// routine to clear the error code
						{ gErrorCode = noErr; }


/*
**	Define Error checking routines
**	These routines return from the current function if the error code is set. (Yuck.)
*/
#define	CheckError()		{ if ( gErrorCode != noErr ) return; }
#define	ReturnError(x)		{ if ( gErrorCode != noErr ) return (x); }
#define	TestPointer(p)		{ if ( not (p) ) SetErrorCode( memFullErr ); }
#define	CheckPointer(p)		{ TestPointer(p); CheckError(); }
#define	ReturnPointer(p,x)	{ if ( not (p) ) SetErrorCode( memFullErr ); ReturnError(x); }


// Backward compatibility for <AssertMacros.h> when building with a pre-10.6 SDK.
#if defined( MAC_OS_X_VERSION_MAX_ALLOWED ) && MAC_OS_X_VERSION_MAX_ALLOWED < 1060
# define __Check( x )			check( x )
# define __Verify( x )			verify( x )
# define __Check_noErr( x )		check_noerr( x )
# define __Verify_noErr( x )	verify_noerr( x )
# define __Debug_String( x )	debug_string( x )
#endif  // MAC_OS_X_VERSION_MAX_ALLOWED < 1060


/*
**	VDebugStr displays (somehow) a debugging message.
**	It has the same arguments as vprintf().
*/
void VDebugStr( const char * format, ... ) PRINTF_LIKE(1, 2);


/*
**	Pre-declare all classes
*/
typedef short DTSCoord;
typedef ushort RGBComp;

class DTSColor;
class DTSPoint;
class DTSLine;
class DTSRect;
class DTSOval;
class DTSRound;
class DTSArc;
//class DTSPolygon;
#ifndef DTS_XCODE
class DTSPattern;
class DTSRegion;
class DTSPicture;
class DTSOffView;
#endif
class DTSImage;
class DTSFocus;
class DTSView;
class DTSWindow;
class DTSControl;
class DTSDate;
class DTSApp;
class DTSKeyFile;
class DTSDlgView;
class DTSDlgList;
class DTSTextField;
class DTSSound;
class DTSFileSpec;

#endif // Local_dts_h
