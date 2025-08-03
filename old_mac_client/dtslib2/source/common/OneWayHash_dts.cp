/*
**	OneWayHash_dts.cp		    dtslib2
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

#if TARGET_API_MAC_OSX || defined( DTS_CLD_Darwin )
# define HAVE_COMMON_DIGEST_H	1
#endif

/*
**	Entry Routine
*/
/*
void DTSOneWayHash( const void * source, size_t len, unsigned char hash[16] );
*/

/* NOTE:
	On MacOSX & Linux, the system provides an implementation of the MD5 algorithm in
	the OpenSSL library (i.e., -lcrypto).
	There's a similar situation on the "new Sun" platform (-lmd5).
	On everything else (Carbon, "old Sun", ...), we have to do all the work ourselves.
	
	This file handles each of those three cases, with the help of two configuration
	macros defined in DTS_Unix.h: HAVE_OPENSSL_MD5 and HAVE_BUILTIN_MD5.
	
	The OpenSSL and new-Sun versions are way down at the bottom. [CommonCrypto, too]
	
	Update 11/11/2012:
	Turns out that if you build the OSX apps against the 10.6 SDK, but then run them on
	an Intel machine using OS 10.4 or 10.5, the apps won't launch, due to what appears
	to be an Xcode issue -- some weird mixup with libcrypto.
	(See <https://stackoverflow.com/questions/2616738/linking-to-libcrypto-for-leopard>.)
	So now instead of using the OpenSSL version, we use the system-supplied one
	in <CommonCrypto/CommonDigest.h>. This is signaled by HAVE_COMMON_DIGEST_H
*/


#if ! defined( HAVE_OPENSSL_MD5 ) && \
	! defined( HAVE_BUILTIN_MD5 ) && \
	! defined( HAVE_COMMON_DIGEST_H )
/*
**	This code was blatantly lifted from http://theory.lcs.mit.edu/~rivest/Rivest-MD5.txt
**	(... and then somewhat C++-ified...)
**
**	Search the internet for MD5 for documentation on the algorithm.
**	Basically it's a one way hash function.
**	The first use is for user authentification.
**	Here's a meaningful message with some random data thrown in.
**	Append the user's password.
**	Hash the whole mess.
**	Send that along with the message.
**	The server does the same procedure and compares the two hashes to authenticate the user.
*/


/* GLOBAL.H - RSAREF types and constants */

/* POINTER defines a generic pointer type */
// typedef void * POINTER;	// -- no longer used

/* UINT2 defines a two byte word */
typedef uint16_t UINT2;

/* UINT4 defines a four byte word */
typedef uint32_t UINT4;


/* MD5.H - header file for MD5C.C */

/* Copyright (C) 1991-2, RSA Data Security, Inc. Created 1991. All
rights reserved.

License to copy and use this software is granted provided that it
is identified as the "RSA Data Security, Inc. MD5 Message-Digest
Algorithm" in all material mentioning or referencing this software
or this function.

License is also granted to make and use derivative works provided
that such works are identified as "derived from the RSA Data
Security, Inc. MD5 Message-Digest Algorithm" in all material
mentioning or referencing the derived work.

RSA Data Security, Inc. makes no representations concerning either
the merchantability of this software or the suitability of this
software for any particular purpose. It is provided "as is"
without express or implied warranty of any kind.

These notices must be retained in any copies of any part of this
documentation and/or software.
 */

/* MD5 context. */
struct MD5_CTX
{
	UINT4 state[4];                                   /* state (ABCD) */
	UINT4 count[2];        /* number of bits, modulo 2^64 (lsb first) */
	unsigned char buffer[64];                         /* input buffer */
};


/* MD5C.C - RSA Data Security, Inc., MD5 message-digest algorithm */

/* Constants for MD5Transform routine. */

#define S11 7
#define S12 12
#define S13 17
#define S14 22
#define S21 5
#define S22 9
#define S23 14
#define S24 20
#define S31 4
#define S32 11
#define S33 16
#define S34 23
#define S41 6
#define S42 10
#define S43 15
#define S44 21

static void MD5Transform( UINT4 [4], const unsigned char [64] );
static void Encode( unsigned char *, const UINT4 *, unsigned int );
static void Decode( UINT4 *, const unsigned char *, unsigned int );

#if 1
// prefer library routines (or compiler-builtins) over the hand-rolled versions
# define MD5_memcpy	std::memcpy
# define MD5_memset	std::memset
#else
static void MD5_memcpy( POINTER, const void *, unsigned int );
static void MD5_memset( POINTER, int, unsigned int );
#endif	// 1


static const unsigned char
PADDING[64] =
{
	0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0
};

/* F, G, H and I are basic MD5 functions. */
#define F(x, y, z) (((x) & (y)) | ((~x) & (z)))
#define G(x, y, z) (((x) & (z)) | ((y) & (~z)))
#define H(x, y, z) ((x) ^ (y) ^ (z))
#define I(x, y, z) ((y) ^ ((x) | (~z)))

/* ROTATE_LEFT rotates x left n bits. */
#ifndef ROTATE_LEFT
# define ROTATE_LEFT(x, n) (((x) << (n)) | ((x) >> (32-(n))))
#endif


/* FF, GG, HH, and II transformations for rounds 1, 2, 3, and 4.
Rotation is separate from addition to prevent recomputation.  */

#define FF(a, b, c, d, x, s, ac) { \
 (a) += F ((b), (c), (d)) + (x) + (UINT4)(ac); \
 (a) = ROTATE_LEFT ((a), (s)); \
 (a) += (b); \
  }
#define GG(a, b, c, d, x, s, ac) { \
 (a) += G ((b), (c), (d)) + (x) + (UINT4)(ac); \
 (a) = ROTATE_LEFT ((a), (s)); \
 (a) += (b); \
  }
#define HH(a, b, c, d, x, s, ac) { \
 (a) += H ((b), (c), (d)) + (x) + (UINT4)(ac); \
 (a) = ROTATE_LEFT ((a), (s)); \
 (a) += (b); \
  }
#define II(a, b, c, d, x, s, ac) { \
 (a) += I ((b), (c), (d)) + (x) + (UINT4)(ac); \
 (a) = ROTATE_LEFT ((a), (s)); \
 (a) += (b); \
  }

/*
**	MD5 initialization.
**	Begins an MD5 operation, writing a new context.
*/
static void
MD5Init( MD5_CTX * context )
{
	context->count[0] = context->count[1] = 0;
	
	/* Load magic initialization constants. */
	context->state[0] = 0x67452301;
	context->state[1] = 0xefcdab89;
	context->state[2] = 0x98badcfe;
	context->state[3] = 0x10325476;
}


/*
**	MD5 block update operation. Continues an MD5 message-digest
**	operation, processing another message block, and updating the
**	context.
*/
static void
MD5Update( MD5_CTX * context, const unsigned char * input, unsigned int inputLen )
{
	unsigned int i, index, partLen;
	
	/* Compute number of bytes mod 64 */
	index = (unsigned int)( (context->count[0] >> 3) & 0x3F );
	
	/* Update number of bits */
	if ( (context->count[0] += ((UINT4)inputLen << 3)) < ((UINT4)inputLen << 3) )
		context->count[1]++;
	
	context->count[1] += ( (UINT4)inputLen >> 29 );
	
	partLen = 64 - index;
	
	/* Transform as many times as possible. */
	if ( inputLen >= partLen )
		{
		MD5_memcpy( &context->buffer[index], input, partLen );
		MD5Transform( context->state, context->buffer );
		
		for ( i = partLen; i + 63 < inputLen; i += 64 )
			MD5Transform( context->state, &input[i] );
		
		index = 0;
		}
	else
		i = 0;
	
	/* Buffer remaining input */
	MD5_memcpy( &context->buffer[index], &input[i], inputLen - i );
}


/*
**	MD5 finalization.
**	Ends an MD5 message-digest operation, writing the
**  the message digest and zeroizing the context.
*/
static void
MD5Final( unsigned char digest[16], MD5_CTX * context )
{
	unsigned char bits[8];
	unsigned int index, padLen;
	
	/* Save number of bits */
	Encode( bits, context->count, 8 );
	
	/* Pad out to 56 mod 64.*/
	index = (unsigned int)( (context->count[0] >> 3) & 0x3f );
	padLen = (index < 56) ? (56 - index) : (120 - index);
	MD5Update( context, PADDING, padLen );
	
	/* Append length (before padding) */
	MD5Update( context, bits, 8 );
	
	/* Store state in digest */
	Encode( digest, context->state, 16 );
	
	/* Zeroize sensitive information.*/
	MD5_memset( context, 0, sizeof( *context ) );
}


/*
**	MD5 basic transformation.
**	Transforms state based on block.
*/
void
MD5Transform( UINT4 state[4], const unsigned char block[64] )
{
	UINT4 a = state[0], b = state[1], c = state[2], d = state[3], x[16];

	Decode( x, block, 64 );
	
	/* Round 1 */
	FF (a, b, c, d, x[ 0], S11, 0xd76aa478); /* 1 */
	FF (d, a, b, c, x[ 1], S12, 0xe8c7b756); /* 2 */
	FF (c, d, a, b, x[ 2], S13, 0x242070db); /* 3 */
	FF (b, c, d, a, x[ 3], S14, 0xc1bdceee); /* 4 */
	FF (a, b, c, d, x[ 4], S11, 0xf57c0faf); /* 5 */
	FF (d, a, b, c, x[ 5], S12, 0x4787c62a); /* 6 */
	FF (c, d, a, b, x[ 6], S13, 0xa8304613); /* 7 */
	FF (b, c, d, a, x[ 7], S14, 0xfd469501); /* 8 */
	FF (a, b, c, d, x[ 8], S11, 0x698098d8); /* 9 */
	FF (d, a, b, c, x[ 9], S12, 0x8b44f7af); /* 10 */
	FF (c, d, a, b, x[10], S13, 0xffff5bb1); /* 11 */
	FF (b, c, d, a, x[11], S14, 0x895cd7be); /* 12 */
	FF (a, b, c, d, x[12], S11, 0x6b901122); /* 13 */
	FF (d, a, b, c, x[13], S12, 0xfd987193); /* 14 */
	FF (c, d, a, b, x[14], S13, 0xa679438e); /* 15 */
	FF (b, c, d, a, x[15], S14, 0x49b40821); /* 16 */
	
	/* Round 2 */
	GG (a, b, c, d, x[ 1], S21, 0xf61e2562); /* 17 */
	GG (d, a, b, c, x[ 6], S22, 0xc040b340); /* 18 */
	GG (c, d, a, b, x[11], S23, 0x265e5a51); /* 19 */
	GG (b, c, d, a, x[ 0], S24, 0xe9b6c7aa); /* 20 */
	GG (a, b, c, d, x[ 5], S21, 0xd62f105d); /* 21 */
	GG (d, a, b, c, x[10], S22,  0x2441453); /* 22 */
	GG (c, d, a, b, x[15], S23, 0xd8a1e681); /* 23 */
	GG (b, c, d, a, x[ 4], S24, 0xe7d3fbc8); /* 24 */
	GG (a, b, c, d, x[ 9], S21, 0x21e1cde6); /* 25 */
	GG (d, a, b, c, x[14], S22, 0xc33707d6); /* 26 */
	GG (c, d, a, b, x[ 3], S23, 0xf4d50d87); /* 27 */
	GG (b, c, d, a, x[ 8], S24, 0x455a14ed); /* 28 */
	GG (a, b, c, d, x[13], S21, 0xa9e3e905); /* 29 */
	GG (d, a, b, c, x[ 2], S22, 0xfcefa3f8); /* 30 */
	GG (c, d, a, b, x[ 7], S23, 0x676f02d9); /* 31 */
	GG (b, c, d, a, x[12], S24, 0x8d2a4c8a); /* 32 */
	
	/* Round 3 */
	HH (a, b, c, d, x[ 5], S31, 0xfffa3942); /* 33 */
	HH (d, a, b, c, x[ 8], S32, 0x8771f681); /* 34 */
	HH (c, d, a, b, x[11], S33, 0x6d9d6122); /* 35 */
	HH (b, c, d, a, x[14], S34, 0xfde5380c); /* 36 */
	HH (a, b, c, d, x[ 1], S31, 0xa4beea44); /* 37 */
	HH (d, a, b, c, x[ 4], S32, 0x4bdecfa9); /* 38 */
	HH (c, d, a, b, x[ 7], S33, 0xf6bb4b60); /* 39 */
	HH (b, c, d, a, x[10], S34, 0xbebfbc70); /* 40 */
	HH (a, b, c, d, x[13], S31, 0x289b7ec6); /* 41 */
	HH (d, a, b, c, x[ 0], S32, 0xeaa127fa); /* 42 */
	HH (c, d, a, b, x[ 3], S33, 0xd4ef3085); /* 43 */
	HH (b, c, d, a, x[ 6], S34,  0x4881d05); /* 44 */
	HH (a, b, c, d, x[ 9], S31, 0xd9d4d039); /* 45 */
	HH (d, a, b, c, x[12], S32, 0xe6db99e5); /* 46 */
	HH (c, d, a, b, x[15], S33, 0x1fa27cf8); /* 47 */
	HH (b, c, d, a, x[ 2], S34, 0xc4ac5665); /* 48 */
	
	/* Round 4 */
	II (a, b, c, d, x[ 0], S41, 0xf4292244); /* 49 */
	II (d, a, b, c, x[ 7], S42, 0x432aff97); /* 50 */
	II (c, d, a, b, x[14], S43, 0xab9423a7); /* 51 */
	II (b, c, d, a, x[ 5], S44, 0xfc93a039); /* 52 */
	II (a, b, c, d, x[12], S41, 0x655b59c3); /* 53 */
	II (d, a, b, c, x[ 3], S42, 0x8f0ccc92); /* 54 */
	II (c, d, a, b, x[10], S43, 0xffeff47d); /* 55 */
	II (b, c, d, a, x[ 1], S44, 0x85845dd1); /* 56 */
	II (a, b, c, d, x[ 8], S41, 0x6fa87e4f); /* 57 */
	II (d, a, b, c, x[15], S42, 0xfe2ce6e0); /* 58 */
	II (c, d, a, b, x[ 6], S43, 0xa3014314); /* 59 */
	II (b, c, d, a, x[13], S44, 0x4e0811a1); /* 60 */
	II (a, b, c, d, x[ 4], S41, 0xf7537e82); /* 61 */
	II (d, a, b, c, x[11], S42, 0xbd3af235); /* 62 */
	II (c, d, a, b, x[ 2], S43, 0x2ad7d2bb); /* 63 */
	II (b, c, d, a, x[ 9], S44, 0xeb86d391); /* 64 */
	
	state[0] += a;
	state[1] += b;
	state[2] += c;
	state[3] += d;
	
	/* Zeroize sensitive information.*/
	MD5_memset( x, 0, sizeof x );
}


/*
**	Encodes input (UINT4) into output (unsigned char).
**	Assumes len is a multiple of 4.
*/
void
Encode( unsigned char * output, const UINT4 * input, unsigned int len )
{
	unsigned int i, j;
	
	for ( i = 0, j = 0; j < len; ++i, j += 4 )
		{
		output[j]   = (unsigned char)(  input[i]        & 0x0ff );
		output[j+1] = (unsigned char)( (input[i] >>  8) & 0x0ff );
		output[j+2] = (unsigned char)( (input[i] >> 16) & 0x0ff );
		output[j+3] = (unsigned char)( (input[i] >> 24) & 0x0ff );
		}
}


/*
**	Decodes input (unsigned char) into output (UINT4).
**	Assumes len is a multiple of 4.
*/
void
Decode( UINT4 * output, const unsigned char * input, unsigned int len )
{
	unsigned int i, j;

	for ( i = 0, j = 0; j < len; ++i, j += 4 )
		{
		output[i] = (   (UINT4) input[j    ] )
		          | ( ( (UINT4) input[j + 1] ) <<  8 )
		          | ( ( (UINT4) input[j + 2] ) << 16 )
		          | ( ( (UINT4) input[j + 3] ) << 24 );
		}
}


// NB: these loops have been supplanted by #defines, above, as suggested.
#if 0

/* Note: Replace "for loop" with standard memcpy if possible. */
void
MD5_memcpy( POINTER output, const void * input, unsigned int len )
{
	for ( unsigned int i = 0; i < len; ++i )
		( (char *) output )[i] = ( (char *) input )[i];
}


/* Note: Replace "for loop" with standard memset if possible. */
void
MD5_memset( POINTER output, int value, unsigned int len )
{
	for ( unsigned int i = 0; i < len; ++i )
		( (char *) output )[i] = (char) value;
}
#endif	// 0


/* Digests a string and prints the result. */
static void
MD5Digest( const unsigned char * source, unsigned int len, unsigned char digest[16] )
{
	MD5_CTX context;
	
	MD5Init( &context );
	MD5Update( &context, source, len );
	MD5Final( digest, &context );
}


// ------ This is the version that's used if we have neither OpenSSL nor NEW_SUN nor OSX 10.4+.
// The real implementation is provided by all the code above this point.

/*
**	DTSOneWayHash()
**
**	cleverly disguise the MD5 algorithm as the DTS one-way hash
*/
void
DTSOneWayHash( const void * source, size_t len, unsigned char hash[16] )
{
	MD5Digest( (const uchar *) source, len, hash );
}
#endif // ! HAVE_OPENSSL_MD5 && ! HAVE_BUILTIN_MD5 && ! HAVE_COMMON_DIGEST_H



// ------ This is the version that's used if we have OpenSSL [i.e. on Linux]

#if HAVE_OPENSSL_MD5
#include <openssl/md5.h>

/*
**	DTSOneWayHash()
**
**	cleverly disguise the MD5 algorithm as the DTS one-way hash
*/
void
DTSOneWayHash( const void * source, size_t len, unsigned char hash[16] )
{
	// use MD5() from <openssl/md5.h>
	::MD5( (const uchar *) source, len, hash );
}

#endif // HAVE_OPENSSL_MD5


// ------ This is the version if we have "new sun"'s builtin MD5
// (which doesn't have the handy MD5() routine, plus there's some
// constness differences)

#if HAVE_BUILTIN_MD5
#include <md5.h>

/*
**	DTSOneWayHash()
**
**	cleverly disguise the MD5 algorithm as the DTS one-way hash
*/
void
DTSOneWayHash( const void * source, size_t len, unsigned char hash[16] )
{
	// compare to MD5Digest(), above
	MD5_CTX context;
	
	MD5Init( &context );
	MD5Update( &context, static_cast<unsigned char *>( const_cast<void *>( source ) ), len );
	MD5Final( hash, &context );
}

#endif // HAVE_BUILTIN_MD5


// ------ This is the version if we have Mac OSX 10.4+ but don't want to use its OpenSSL.

#if HAVE_COMMON_DIGEST_H
# include <AvailabilityMacros.h>
# include <CommonCrypto/CommonDigest.h>

// MD5 & co. are deprecated in the 10.15 SDK
// but we still need to use it, until we can convert _all_ our clients to SHA-256 or whatever
# if MAC_OS_X_VERSION_MIN_REQUIRED >= 101500
#  pragma GCC diagnostic push
#  pragma GCC diagnostic ignored "-Wdeprecated-declarations"
# endif

/*
**	DTSOneWayHash()
**
**	cleverly disguise the MD5 algorithm as the DTS one-way hash
*/
void
DTSOneWayHash( const void * source, size_t len, unsigned char hash[16] )
{
#if MAC_OS_X_VERSION_MIN_REQUIRED >= 1050
	// the 10.5 SDK added this more convenient "one-shot" interface
	CC_MD5( source, static_cast<CC_LONG>( len ), hash );
#else
	// compare to MD5Digest(), above
	CC_MD5_CTX context;
	
	CC_MD5_Init( &context );
	CC_MD5_Update( &context, source, static_cast<CC_LONG>( len ) );
	CC_MD5_Final( hash, &context );
#endif  // 10.5
}

# if MAC_OS_X_VERSION_MIN_REQUIRED >= 101500
#  pragma GCC diagnostic pop
# endif

#endif  // HAVE_COMMON_DIGEST_H

