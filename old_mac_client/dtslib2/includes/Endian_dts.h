/*
**	Endian_dts.h		dtslib2
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

#ifndef Endian_dts_h
#define Endian_dts_h

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "Local_dts.h"


/*
**	handle endian-ness
*/
extern ushort SwapEndian( ushort x );
extern uint	  SwapEndian( uint x );
#ifndef __LP64__
	// We never have reason to swap 64-bit entities.
	// Therefore when compiling for LP64, all of the 'long/ulong'-flavored functions
	// in this file are ifdef'ed out, and any client code that attempts to swap a 64-bit
	// object will raise an error. The 'int/uint' flavors handle 32-bit objects, as expected.
	
	// By contrast, in 32-bit environments, even though 'int' and 'long' are both 32 bits
	// wide, the types are not synonymous according to the rules of C. So we must provide
	// both flavors (which end up doing the exact same job), to keep the compiler happy.
	
	// The 16-bit 'short' flavors are not affected by the LP64 setting.
extern ulong  SwapEndian( ulong x );
#endif

inline short SwapEndian( short x ) { return short( SwapEndian( ushort(x) ) ); }
inline   int SwapEndian(   int x ) { return   int( SwapEndian(   uint(x) ) ); }
#ifndef __LP64__
inline  long SwapEndian(  long x ) { return  long( SwapEndian(  ulong(x) ) ); }
#endif


#if DTS_BIG_ENDIAN
inline  short	BigToNativeEndian(  short x ) { return x; }
inline  short	NativeToBigEndian(  short x ) { return x; }
inline ushort	BigToNativeEndian( ushort x ) { return x; }
inline ushort	NativeToBigEndian( ushort x ) { return x; }
inline    int	BigToNativeEndian(    int x ) { return x; }
inline    int	NativeToBigEndian(    int x ) { return x; }
inline   uint	BigToNativeEndian(   uint x ) { return x; }
inline   uint	NativeToBigEndian(   uint x ) { return x; }
# ifndef __LP64__ 
inline   long	BigToNativeEndian(   long x ) { return x; }
inline   long	NativeToBigEndian(   long x ) { return x; }
inline  ulong	BigToNativeEndian(  ulong x ) { return x; }
inline  ulong	NativeToBigEndian(  ulong x ) { return x; }
# endif  // ! __LP64__
#else  // little-endian
inline  short	BigToNativeEndian(  short x ) { return static_cast<short>( ntohs( x ) ); }
inline  short	NativeToBigEndian(  short x ) { return static_cast<short>( htons( x ) ); }
inline ushort	BigToNativeEndian( ushort x ) { return ntohs( x ); }
inline ushort	NativeToBigEndian( ushort x ) { return htons( x ); }
inline    int	BigToNativeEndian(    int x ) { return static_cast<int>( ntohl( x ) ); }
inline    int	NativeToBigEndian(    int x ) { return static_cast<int>( htonl( x ) ); }
inline   uint	BigToNativeEndian(   uint x ) { return ntohl( x ); }
inline   uint	NativeToBigEndian(   uint x ) { return htonl( x ); }
# ifndef __LP64__
inline   long	BigToNativeEndian(   long x ) { return static_cast<long>( ntohl( x ) ); }
inline   long	NativeToBigEndian(   long x ) { return static_cast<long>( ( x ) ); }
inline  ulong	BigToNativeEndian(  ulong x ) { return ntohl( x ); }
inline  ulong	NativeToBigEndian(  ulong x ) { return htonl( x ); }
# endif  // __LP64__
#endif	// DTS_BIG_ENDIAN

#endif	// Endian_dts_h

