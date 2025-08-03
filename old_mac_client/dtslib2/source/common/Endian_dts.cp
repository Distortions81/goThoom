/*
**	Endian_dts.cp		dtslib2
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

// Make intrinsics available to GCC
#if __POWERPC__
# include <ppc_intrinsics.h>
#endif

// similar for linux
#if defined( __linux )
# include <byteswap.h>
#endif


/*
**	Entry Routines
*/
/*
ulong	SwapEndian( ulong x );
uint	SwapEndian( uint x );
ushort	SwapEndian( ushort x );
*/


/*
**	SwapEndian()
**
**	swap the byte order of a 32-bit integer (type 'long', when not in 64-bit mode)
**	
**	I leave this undefined under LP64 because nothing in our codebase
**	has any business trying to swap 64-bit entities; I want the linker
**	to nip in the bud any such attempts.
**	
**	On 32-bit platforms, this routine is identical to SwapEndian(int),
**	but we must nevertheless supply both, since types 'int' and 'long'
**	are not synonymous to the compiler.
*/
#ifndef __LP64__
ulong
SwapEndian( ulong x )
{
# if __POWERPC__
	return __lwbrx( &x, 0 );
# elif TARGET_API_MAC_OSX
	return Endian32_Swap( x );
# elif defined( __linux )
	return bswap_32( uint32_t( x ) );
# else
 	return ( (x) >> 24 ) | ( (x) << 24 )
		 | ( ((x) >> 8) & 0x0000FF00 )
		 | ( ((x) << 8) & 0x00FF0000 );
# endif
}
#endif  // __LP64__


/*
**	SwapEndian()
**
**	swap the byte order of a 32-bit integer (type 'int')
*/
uint
SwapEndian( uint x )
{
#if __POWERPC__
	return __lwbrx( &x, 0 );
#elif TARGET_API_MAC_OSX
	return Endian32_Swap( x );
#elif defined( __linux )
	return bswap_32( x );
#else
	return ( (x) >> 24 ) | ( (x) << 24 )
		 | ( ((x) >> 8) & 0x0000FF00 )
		 | ( ((x) << 8) & 0x00FF0000 );
#endif
}


/*
**	SwapEndian()
**
**	swap the byte order of a 16-bit integer
*/
ushort
SwapEndian( ushort x )
{
#if __POWERPC__
	return __lhbrx( &x, 0 );
#elif TARGET_API_MAC_OSX
	return Endian16_Swap( x );
#elif defined( __linux )
	return bswap_16( x );
#else
//	return short( ( ushort(x) >> 8 ) | ( ushort(x) << 8 ) );
//	return ((((x) >> 8) & 0xff) | (((x) & 0xff) << 8));
	return ushort( (((x) & 0xFF00) >> 8) | (((x) & 0x00FF) << 8) );
#endif
}


