/*
**	Utilities_dts.cp		dtslib2
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

#include "Utilities_dts.h"


/*
**	GetRandom();
**	GetRandSeed();
**	SetRandSeed();
**	
**	StringToUpper();
**	StringToLower();
**	StringCopyPad();
**	StringCopyDst();
**	StringCopySafe();
**	BufferToStringSafe();
**	
**	int  snprintfx( char * buff, size_t size, const char * format, ... );
**	int vsnprintfx( char * buff, size_t size, const char * format, va_list params );
*/

/*
**	Internal Variables
*/
static uint32_t		gSeed;


// This is our new RNG, the so-called "Mersenne Twister".
// See <http://www.math.sci.hiroshima-u.ac.jp/~m-mat/MT/emt.html>
// Copyright (C) 1997 - 2002, Makoto Matsumoto and Takuji Nishimura

class MTRandom
{
public:
	// constructor, with initial seeds
	MTRandom( uint32_t seed )			{ Reseed( seed ); }
	
	// There's provision for supplying more than 32 bits of seed material,
	// but neither DTSLib nor its clients have any need for that.
//	MTRandom( const uint32_t keys[], size_t numKeys )	{ Reseed( keys, numKeys ); }
	
	uint32_t	RandomInt();				// generates a random number on [0, UINT_MAX]
	uint32_t	RandomInt( uint32_t range );	// generates a random number on [0, range);
	
protected:
	static const int	MT_N = 624;		// size of state vector
	static const int	MT_M = 397;		// period factor
	
	uint32_t	mt[ MT_N ];				// state vector
	int			mti;					// state index (if >MN_N, must cook up a fresh batch)
	
	void		Reseed( uint32_t s );
//	void		Reseed( const uint32_t keys[], size_t numKeys );
	
				// Needs friendship, to reseed
	friend void	SetRandSeed( ulong s );
};


/*
**	MTRandom::Reseed()
**	(re)initialize the RNG using a given 32-bit seed
*/
void
MTRandom::Reseed( uint32_t s )
{
	// just in case
	if ( 0 == s )
		s = 19650218U;
	
	gSeed = mt[ 0 ] = s;
	for ( mti = 1; mti < MT_N; ++mti )
		mt[ mti ] = (1812433253U * (mt[mti - 1] ^ (mt[mti - 1] >> 30)) + uint(mti));
}


#if 0	// not used
/*
**	MTRandom::Reseed()
**	(re)initialize the RNG using lots of seed material
*/
void
MTRandom::Reseed( const uint32_t keys[], size_t numKeys )
{
	Reseed( 19650218U );
	int i = 1;
	int j = 0;
	int k = ( MT_N > numKeys ) ? MT_N : numKeys;
	for ( ; k; --k )
		{
		mt[i] = (mt[i] ^ ((mt[i-1] ^ (mt[i-1] >> 30)) * 1664525U)) + keys[j] + j;
		++i;
		++j;
		if ( i >= MT_N )
			{
			mt[0] = mt[MT_N - 1];
			i = 1;
			}
		if ( j >= numKeys )
			j = 0;
		}
	for ( k = MT_N - 1; k; --k)
		{
		mt[i] = (mt[i] ^ ((mt[i-1] ^ (mt[i-1] >> 30)) * 1566083941U)) - i;
		++i;
		if ( i >= MT_N )
			{
			mt[0] = mt[MT_N - 1];
			i = 1;
			}
		}
	
	mt[0] = 0x80000000U;	// MSB is 1; assuring non-zero initial array
}
#endif // notused


/*
**	MTRandom::RandomInt()
**	Generate a single random integer, in the range [0, UINT_MAX]
*/
uint32_t
MTRandom::RandomInt()
{
	const uint32_t mag01[2] = { 0, 0x9908B0DFU };
	const uint32_t UPPER_MASK = 0x80000000U;		// upper w - r bits (w = 32, r = 31)
	const uint32_t LOWER_MASK = 0x7FFFFFFFU;		// lower r bits
	
	uint32_t y;

	if ( mti >= MT_N )	// generate a new batch of MT_N random words
		{
		int kk;
		
		for ( kk = 0; kk < MT_N - MT_M; ++kk )
			{
		    y = (mt[kk] & UPPER_MASK) | (mt[kk + 1] & LOWER_MASK );
		    mt[kk] = mt[kk + MT_M] ^ (y >> 1) ^ mag01[y & 1];
			}
		for ( ; kk < MT_N - 1; ++kk )
			{
		    y = (mt[kk] & UPPER_MASK) | (mt[kk + 1] & LOWER_MASK);
		    mt[kk] = mt[kk + (MT_M - MT_N)] ^ (y >> 1) ^ mag01[y & 1];
			}
		y = (mt[MT_N - 1] & UPPER_MASK) | (mt[0] & LOWER_MASK);
		mt[MT_N - 1] = mt[MT_M - 1] ^ (y >> 1) ^ mag01[y & 1];
		
		mti = 0;
    	}

	y = mt[ mti++ ];
	
	// Tempering 
	y ^= (y >> 11);
	y ^= (y <<  7) & 0x9D2C5680U;
	y ^= (y << 15) & 0xEFC60000U;
	y ^= (y >> 18);

	return y;
}


/*
**	MTRandom::RandomInt()
**	Generate a single random number, in the half-open interval [0, r)
*/
uint32_t
MTRandom::RandomInt( uint32_t range )
{
	// just in case
	if ( 0 == range )
		return 0;
	
#if 1
	// simple but maybe not-so-good method.
	// division is slow, and the low-order bits might not be
	// perfectly well-distributed, modulo range.
	// on the other hand, I'm not sure it's _that_ much worse than
	// the alternate method below (which, after all, might have a
	// potential worst-case running time of years & years!)...
	
	return RandomInt() % range;
#else
	// theoretically better method, using ideas from an alternate Mersenne Twister
	// implementation we looked at, from <http://www.agner.org/>
	
	// Find which bits are used in 'range'
	// at the end, 'used' is the smallest (2^n - 1) that's not less than 'range'.
	uint32_t used = range;
	used |= used >> 1;
	used |= used >> 2;
	used |= used >> 4;
	used |= used >> 8;
	used |= used >> 16;
	
	// Draw numbers until we find an acceptable one, i.e., within [0,range)
	uint32_t i;
	do
		i = RandomInt() & used;		// toss unused bits to shorten search
	while( i >= range );			// (in the worst case, this loop might _never_ exit?)
	return i;
#endif // 1
}


/*
**	GetRNG()
**
**	We only keep a single instance of the MTRandom class on hand; this
**	is the accessor for it. This is safer than static initialization, because
**	we very much want gSeed to be ready before constructing the RNG instance.
*/
static MTRandom&
GetRNG()
{
	static MTRandom theRNG( gSeed );	// singleton
	return theRNG;
}


/*
**	GetRandom()
**	return a random integer from 0 to (range - 1), inclusive
*/
ulong
GetRandom( ulong range )
{
	return GetRNG().RandomInt( static_cast<uint32_t>( range ) );
}


/*
**	GetRandSeed()
**	return the current random seed
**	With SetRandSeed(), permits generation of repeatable sequences of random numbers.
*/
ulong
GetRandSeed()
{
	return gSeed;
}


/*
**	SetRandSeed()
**	(re)initialize the RNG with a given seed
*/
void
SetRandSeed( ulong s )
{
	GetRNG().Reseed( static_cast<uint32_t>( s ) );
}


#if 0
// This is the old, original DTSLib RNG algorithm.
// It's pretty fast, but not very reliable.

/*
**	GetRandom()
*/
ulong
GetRandom( ulong range )
{
	ulong seed = gSeed;
	if ( 0 == seed )
		{
#if defined( DTS_Mac )
		seed = ::Random() & 0x0FF;
#else
		seed = random() & 0x0FF;
#endif
		if ( 0 == seed )
			seed = 0x0A5;
		seed |= 0xFE1F6600;
		gSeed = seed;
		}
	
	ulong value = 0;
	if ( range != 0 )
		{
		value  = ( seed >> 8 ) | ( seed << 24 );
		value %= range;
		gSeed  = seed * 0x963C5A93;
		}
	
	return value;
}


/*
**	GetRandSeed()
*/
ulong
GetRandSeed()
{
	// make sure the random seed is initialized
	if ( 0 == gSeed )
		GetRandom( 1 );
	
	return gSeed;
}


/*
**	SetRandSeed()
*/
void
SetRandSeed( ulong seed )
{
	if ( seed != 0 )
		gSeed = seed;
}
#endif // old RNG

#pragma mark -


/*
**	StringToUpper()
**
**	convert a string to upper case. Specifically, changes 'a'..'z' to 'A'..'Z';
**	this routine doesn't modify (or even understand) any characters > 0x7F, because
**	to do so responsibly requires a more sophisticated approach to encodings,
**	locales, and character sets.
*/
void
StringToUpper( char * str )
{
	for (;;)
		{
		char ch = *str;
		if ( 0 == ch )
			break;
		if ( ch >= 'a' && ch <= 'z' )
			*str = static_cast<char>( ch - 'a' + 'A' );
		++str;
		}
}


/*
**	StringToLower()
**
**	convert a string to lowercase. See above.
*/
void
StringToLower( char * str )
{
	for (;;)
		{
		char ch = *str;
		if ( 0 == ch )
			break;
		if ( ch >= 'A' && ch <= 'Z' )
			*str = static_cast<char>( ch - 'a' + 'A' );
		++str;
		}
}


/*
**	StringCopyPad()
**
**	copy a string and pad remaining space with zeros
**	truncate the string if necessary
**	return a pointer to the next byte in the source
*/
const char *
StringCopyPad( char * dst, const char * src, size_t size )
{
	// point at the last byte in the destination buffer
	const char * limit = dst + size - 1;
	
	// start moving bytes
	for(;;)
		{
		// have we reached the end of the destination buffer?
		if ( dst >= limit )
			{
			// yes, terminate it
			*dst = 0;
			
			// scan past the remaining source string
			while ( *src++ )
				;
			return src;
			}
		
		// have we reached the end of the source string?
		char ch = *src++;
		if ( 0 == ch )
			{
			// yes, fill the remaining destination buffer with zeros
			if ( dst <= limit )
				memset( dst, 0, static_cast<size_t>( limit - dst + 1 ) );
			return src;
/*
			for(;;)
				{
				*dst = 0;
				if ( dst >= limit )
					return src;
				++dst;
				}
*/
			}
		
		// copy the character
		*dst++ = ch;
		}
}


/*
**	StringCopyDst()
**
**	copy a string and return a pointer
**	to the first (destination) byte after the null
*/
char *
StringCopyDst( char * dst, const char * src )
{
	for(;;)
		{
		char ch = *src;
		*dst++ = ch;
		if ( 0 == ch )
			return dst;
		++src;
		}
}


#if ! HAVE_STRLCPY && ! TARGET_API_MAC_OSX
/*
**	StringCopySafe()
**
**	copy a string & truncate if necessary
*/
void
StringCopySafe( char * dst, const char * src, size_t size )
{
	// safety check
	if ( 0 == size )
		return;
	
	// point at the last byte in the destination buffer
	const char * limit = dst + size - 1;
	
	// start moving bytes
	for(;;)
		{
		// have we reached the end of the destination buffer?
		if ( dst >= limit )
			{
			// yes, terminate it
			*dst = 0;
			return;
			}
		
		// copy the character
		char ch = *src++;
		*dst++ = ch;
		
		// have we reached the end of the source string?
		if ( 0 == ch )
			return;
		}
}
#endif  // ! HAVE_STRLCPY / TARGET_API_MAC_OSX


/*
**	BufferToStringSafe()
**
**	copy an array of characters to a string in a buffer.
**	the source is not necessarily nul-terminated, but
**	we ensure that the destination is.
*/
void
BufferToStringSafe( char * dst, size_t dstsize, const char * src, size_t srcsize )
{
	// safety check
	if ( dstsize <= 0 )
		return;
	
	// leave room for the terminating null
	--dstsize;
	
	// trim the source
	if ( srcsize > dstsize )
		srcsize = dstsize;
	
	// copy characters
	for (  ;  srcsize > 0;  --srcsize )
		{
		char ch = *src;
		if ( 0 == ch )
			break;
		*dst++ = ch;
		++src;
		}
	
	// add the terminating null
	*dst = 0;
}


/*
**	snprintfx()
**
**	safe version of sprintf()
**	don't overwrite the end of the buffer
**	and return the number of characters actually put into the buffer.
**	(Standard snprintf() returns the # of chars it _would have_ written,
**	assuming a sufficiently-large buffer. In other words...
**		snprintf ( dst, 2, "abcde" ) == 6	// strlen("abcde") + 1
**		snprintfx( dst, 2, "abcde" ) == 2	// only wrote "a\0"
*/
int
snprintfx( char * buff, size_t size, const char * format, ... )
{
	va_list params;
	va_start( params, format );
	
	int len = vsnprintfx( buff, size, format, params );
	
	va_end( params );
	
	return len;
}


/*
**	vsnprintfx()
**
**	safe version of vsprintf()
**	don't overwrite the end of the buffer
**	and return the number of characters actually put into the buffer (see above).
*/
int
vsnprintfx( char * buff, size_t size, const char * format, va_list params )
{
	int len = vsnprintf( buff, size, format, params );
	
	--size;
	if ( len > long( size ) )
		len = int( size );
	
	return len;
}


