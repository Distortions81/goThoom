/*
**	Encryption_dts.cp		dtslib2
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

#include "Encryption_dts.h"

using std::memset;
using std::memcpy;


/*
**	Entry Routines
*/
/*
DTSError DTSEncode( const void * plaintext, void * ciphertext, size_t len, const void * key, size_t keylen );
DTSError DTSDecode( const void * ciphertext, void * plaintext, size_t len, const void * key, size_t keylen );
*/


/*
	This code was blatantly lifted from <http://www.counterpane.com/twofish.html>
	Search the internet for twofish for documentation on the algorithm.

	Code subsequently modified by DeltaTao (wearing very thick gloves, and 
	holding our noses tightly shut) to try to fix the worst idiocies, malfeatures,
	brain damage, and general breakage. The angel choir took one look at this stuff
	and fled to Elysium. Despite our best efforts, the code isn't blindingly beautiful
	at all: just blindingly atrocious. It might still make your eyes melt --
	but blame the folks below for that, not us!
*/



/***************************************************************************
	PLATFORM.H	-- Platform-specific defines for TWOFISH code

	Submitters:
		Bruce Schneier, Counterpane Systems
		Doug Whiting,	Hi/fn
		John Kelsey,	Counterpane Systems
		Chris Hall,		Counterpane Systems
		David Wagner,	UC Berkeley
			
	Code Author:		Doug Whiting,	Hi/fn
		
	Version  1.00		April 1998
		
	Copyright 1998, Hi/fn and Counterpane Systems.  All rights reserved.
		
	Notes:
		*	Tab size is set to 4 characters in this file

***************************************************************************/

/* use intrinsic rotate if possible */
#if defined( __POWERPC__ ) && defined( __GNUC__ )
# include <ppc_intrinsics.h>
# define ROL(x,n)	__rlwinm( (x), (n), 0, 31 )
# define ROR(x,n)	__rlwinm( (x), (32 - n), 0, 31 )
#endif	// mwerks, gcc, ppc

#if (0) && defined(__BORLANDC__) && (__BORLANDC__ >= 0x462)
# error "!!!This does not work for some reason!!!"
//#include	<stdlib.h>					/* get prototype for _lrotl() , _lrotr() */
# pragma inline __lrotl__
# pragma inline __lrotr__
# undef	ROL								/* get rid of inefficient definitions */
# undef	ROR
# define	ROL(x,n)	__lrotl__(x,n)		/* use compiler intrinsic rotations */
# define	ROR(x,n)	__lrotr__(x,n)
#endif

#ifdef _MSC_VER
//#include	<stdlib.h>					/* get prototypes for rotation functions */
# undef	ROL
# undef	ROR
# pragma intrinsic(_lrotl,_lrotr)		/* use intrinsic compiler rotations */
# define	ROL(x,n)	_lrotl(x,n)			
# define	ROR(x,n)	_lrotr(x,n)
#endif

// fallback rotates if no other intrinsics available
#ifndef ROL
//# define	ROL(x,n) ( ((x) << ((n) & 0x1F)) | ((x) >> (32-((n) & 0x1F))) )
# define	ROL(x,n) ( ((x) << (n)) | ((x) >> ( 32-(n) )) )
#endif
#ifndef ROR
//# define	ROR(x,n) ( ((x) >> ((n) & 0x1F)) | ((x) << (32-((n) & 0x1F))) )
# define	ROR(x,n) ( ((x) >> (n)) | ((x) << (32-(n))) )
#endif


#ifndef _M_IX86
# ifdef	__BORLANDC__
#  define	_M_IX86				300		/* make sure this is defined for Intel CPUs */
# endif
#endif

#if defined( _M_IX86 ) || defined( __i386__ )
# define	LittleEndian		1		/* e.g., 1 for Pentium, 0 for 68K */
# define	ALIGN32				0		/* need dword alignment? (no for Pentium) */
#elif defined ( __LITTLE_ENDIAN__ ) || defined( _LITTLE_ENDIAN ) || DTS_LITTLE_ENDIAN
# define	LittleEndian		1		/* e.g., 1 for Pentium, 0 for 68K */
# define	ALIGN32				1		/* need dword alignment? (no for Pentium) */
	// ... but it can't -hurt- to be aligned, can it?
#else	/* BigEndian platforms */
# define	LittleEndian		0		/* (assume big endian */
# define	ALIGN32				1		/* (assume need alignment for non-Intel) */
#endif

// Some code below tries to access various BYTE tables as if they were
// actually tables of DWORDs. That can cause alignment problems for some CPUs,
// ranging from invisible slowdowns all the way to outright bus faults.
// Fixing the code is too distasteful to contemplate, but we can at least
// treat the symptom, by enforcing proper alignment. Thus this next macro.

#if defined( __GNUC__ )
# define ALIGN_ME	__attribute__(( __aligned__ ))
// GCC supports the attribute(aligned) extension.
// Leaving out the alignment factor means machine's "maximum useful alignment".
#else
// Just a no-op for other compilers
# define ALIGN_ME
#endif


#if LittleEndian
# define	Bswap(x)			(x)		/* NOP for little-endian machines */
# define	ADDR_XOR			0		/* NOP for little-endian machines */
#else
# if defined( __POWERPC__ ) && defined( __GNUC__ )
//#  define	Bswap(x)			__lwbrx( (void *) &x, 0 )
inline unsigned long
Bswap( unsigned long x )
{
	return __lwbrx( &x, 0 );
}
# else
#  define	Bswap(x)			((ROR(x,8) & 0xFF00FF00) | (ROL(x,8) & 0x00FF00FF))
# endif
# define	ADDR_XOR			3		/* convert byte address in dword */
#endif  // LittleEndian

/*	Macros for extracting bytes from dwords (correct for endianness) */
 /* pick bytes out of a dword */
#define	_b(x,N)	( (reinterpret_cast<BYTE *>( &x ))[((N) & 3) ^ ADDR_XOR] )

#define	b0(x)	_b( x, 0 )		/* extract LSB of DWORD */
#define	b1(x)	_b( x, 1 )
#define	b2(x)	_b( x, 2 )
#define	b3(x)	_b( x, 3 )		/* extract MSB of DWORD */


/* aes.h */

/* ---------- See examples at end of this file for typical usage -------- */

/* AES Cipher header file for ANSI C Submissions
	Lawrence E. Bassham III
	Computer Security Division
	National Institute of Standards and Technology

	This sample is to assist implementers developing to the
Cryptographic API Profile for AES Candidate Algorithm Submissions.
Please consult this document as a cross-reference.
	
	ANY CHANGES, WHERE APPROPRIATE, TO INFORMATION PROVIDED IN THIS FILE
MUST BE DOCUMENTED. CHANGES ARE ONLY APPROPRIATE WHERE SPECIFIED WITH
THE STRING "CHANGE POSSIBLE". FUNCTION CALLS AND THEIR PARAMETERS
CANNOT BE CHANGED. STRUCTURES CAN BE ALTERED TO ALLOW IMPLEMENTERS TO
INCLUDE IMPLEMENTATION SPECIFIC INFORMATION.
*/

/* Includes:
	Standard include files
*/

//#include	<stdio.h>
//#include	"platform.h"			/* platform-specific defines */

/*	Defines:
		Add any additional defines you need
*/

#define 	DIR_ENCRYPT 	0 		/* Are we encrpyting? */
#define 	DIR_DECRYPT 	1 		/* Are we decrpyting? */
#define 	MODE_ECB 		1 		/* Are we ciphering in ECB mode? */
#define 	MODE_CBC 		2 		/* Are we ciphering in CBC mode? */
#define 	MODE_CFB1 		3 		/* Are we ciphering in 1-bit CFB mode? */

#if !defined(TRUE)
#define 	TRUE 			1
#endif
#if !defined(FALSE)
#define 	FALSE 			0
#endif
#if !defined(NULL)
#define		NULL			0
#endif
#if !defined(noErr)
#define		noErr			0
#endif

#define 	BAD_KEY_DIR 		-1	/* Key direction is invalid (unknown value) */
#define 	BAD_KEY_MAT 		-2	/* Key material not of correct length */
#define 	BAD_KEY_INSTANCE 	-3	/* Key passed is not valid */
#define 	BAD_CIPHER_MODE 	-4 	/* Params struct passed to cipherInit invalid */
#define 	BAD_CIPHER_STATE 	-5 	/* Cipher in wrong state (e.g., not initialized) */

/* CHANGE POSSIBLE: inclusion of algorithm specific defines */
/* TWOFISH specific definitions */
#define		MAX_KEY_SIZE		64	/* # of ASCII chars needed to represent a key */
#define		MAX_IV_SIZE			16	/* # of bytes needed to represent an IV */
#define		BAD_INPUT_LEN		-6	/* inputLen not a multiple of block size */
#define		BAD_PARAMS			-7	/* invalid parameters */
#define		BAD_IV_MAT			-8	/* invalid IV text */
#define		BAD_ENDIAN			-9	/* incorrect endianness define */
#define		BAD_ALIGN32			-10	/* incorrect 32-bit alignment */

#define		BLOCK_SIZE			128	/* number of bits per block */
#define		MAX_ROUNDS			 16	/* max # rounds (for allocating subkey array) */
#define		ROUNDS_128			 16	/* default number of rounds for 128-bit keys*/
#define		ROUNDS_192			 16	/* default number of rounds for 192-bit keys*/
#define		ROUNDS_256			 16	/* default number of rounds for 256-bit keys*/
#define		MAX_KEY_BITS		256	/* max number of bits of key */
#define		MIN_KEY_BITS		128	/* min number of bits of key (zero pad) */
#define		VALID_SIG	 0x48534946	/* initialization signature ('FISH') */
#define		MCT_OUTER			400	/* MCT outer loop */
#define		MCT_INNER		  10000	/* MCT inner loop */
#define		REENTRANT			  0	/* nonzero forces reentrant code (slightly slower) */

#define		INPUT_WHITEN		0	/* subkey array indices */
#define		OUTPUT_WHITEN		( INPUT_WHITEN + BLOCK_SIZE/32)
#define		ROUND_SUBKEYS		(OUTPUT_WHITEN + BLOCK_SIZE/32)	/* use 2 * (# rounds) */
#define		TOTAL_SUBKEYS		(ROUND_SUBKEYS + 2*MAX_ROUNDS)

/* Typedefs:
	Typedef'ed data storage elements. Add any algorithm specific
	parameters at the bottom of the structs as appropriate.
*/

typedef unsigned char BYTE;
typedef	uint32_t DWORD;				/* 32-bit unsigned quantity */
typedef DWORD fullSbox[4][256];

/* The structure for key information */
typedef struct 
	{
	BYTE direction;					/* Key used for encrypting or decrypting? */
#if ALIGN32
	BYTE dummyAlign[3];				/* keep 32-bit alignment */
#endif
	int  keyLen;					/* Length of the key */
	char keyMaterial[MAX_KEY_SIZE+4];/* Raw key data in ASCII */

	/* Twofish-specific parameters: */
	DWORD keySig;					/* set to VALID_SIG by makeKey() */
	int	  numRounds;				/* number of rounds in cipher */
	DWORD key32[MAX_KEY_BITS/32];	/* actual key bits, in dwords */
	DWORD sboxKeys[MAX_KEY_BITS/64];/* key bits used for S-boxes */
	DWORD subKeys[TOTAL_SUBKEYS];	/* round subkeys, input/output whitening bits */
#if REENTRANT
	fullSbox sBox8x32;				/* fully expanded S-box */
  #if defined(COMPILE_KEY) && defined(USE_ASM)
	#undef	VALID_SIG
	#define	VALID_SIG	 0x504D4F43		/* 'COMP':  C is compiled with -DCOMPILE_KEY */
	DWORD cSig1;					/* set after first "compile" (zero at "init") */
	void *encryptFuncPtr;			/* ptr to asm encrypt function */
	void *decryptFuncPtr;			/* ptr to asm decrypt function */
	DWORD codeSize;					/* size of compiledCode */
	DWORD cSig2;					/* set after first "compile" */
	BYTE  compiledCode[5000];		/* make room for the code itself */
  #endif
#endif	/* REENTRANT */
	} keyInstance;

/* The structure for cipher information */
typedef struct 
	{
	BYTE  mode;						/* MODE_ECB, MODE_CBC, or MODE_CFB1 */
#if ALIGN32
	BYTE dummyAlign[3];				/* keep 32-bit alignment */
#endif
	BYTE  IV[MAX_IV_SIZE];			/* CFB1 iv bytes  (CBC uses iv32) */

	/* Twofish-specific parameters: */
	DWORD cipherSig;				/* set to VALID_SIG by cipherInit() */
	DWORD iv32[BLOCK_SIZE/32];		/* CBC IV bytes arranged as dwords */
	} cipherInstance;

/* Function protoypes */
static int makeKey( keyInstance * key, BYTE direction, int keyLen, const char * keyMaterial );

static int cipherInit( cipherInstance * cipher, BYTE mode, const char * IV );

static int blockEncrypt( cipherInstance * cipher, keyInstance * key, const BYTE * input,
				int inputLen, BYTE * outBuffer );

static int blockDecrypt( cipherInstance * cipher, keyInstance * key, const BYTE * input,
				int inputLen, BYTE * outBuffer );

static int	reKey( keyInstance * key );	/* do key schedule using modified key.keyDwords */

/* API to check table usage, for use in ECB_TBL KAT */
#define		TAB_DISABLE			0
#define		TAB_ENABLE			1
#define		TAB_RESET			2
#define		TAB_QUERY			3
#define		TAB_MIN_QUERY		50
// static int TableOp(int op);



#if BLOCK_SIZE == 128			/* optimize block copies */
#define		Copy1(d,s,N)	( (DWORD *)(d) )[ N ] = ( (DWORD *)(s) )[ N ]
#define		BlockCopy(d,s)	{ Copy1(d,s,0); Copy1(d,s,1); Copy1(d,s,2); Copy1(d,s,3); }
#else
#define		BlockCopy(d,s)	{ memcpy( d, s, BLOCK_SIZE/8 ); }
#endif


#ifdef TEST_2FISH
/*						----- EXAMPLES -----

Unfortunately, the AES API is somewhat clumsy, and it is not entirely
obvious how to use the above functions.  In particular, note that
makeKey() takes an ASCII hex nibble key string (e.g., 32 characters
for a 128-bit key), which is rarely the way that keys are internally
represented.  The reKey() function uses instead the keyInstance.key32
array of key bits and is the preferred method.  In fact, makeKey()
initializes some internal keyInstance state, then parse the ASCII
string into the binary key32, and calls reKey().  To initialize the
keyInstance state, use a 'dummy' call to makeKey(); i.e., set the
keyMaterial parameter to NULL.  Then use reKey() for all key changes.
Similarly, cipherInit takes an IV string in ASCII hex, so a dummy setup
call with a null IV string will skip the ASCII parse.  

Note that CFB mode is not well tested nor defined by AES, so using the
Twofish MODE_CFB it not recommended.  If you wish to implement a CFB mode,
build it external to the Twofish code, using the Twofish functions only
in ECB mode.

Below is a sample piece of code showing how the code is typically used
to set up a key, encrypt, and decrypt.  Error checking is somewhat limited
in this example.  Pseudorandom bytes are used for all key and text.

If you compile TWOFISH2.C or TWOFISH.C as a DOS (or Windows Console) app
with this code enabled, the test will be run.  For example, using
Borland C, you would compile using:
  BCC32 -DTEST_2FISH twofish2.c
to run the test on the optimized code, or
  BCC32 -DTEST_2FISH twofish.c
to run the test on the pedagogical code.

*/

//#include <stdio.h>
//#include <stdlib.h>
//#include <time.h>
//#include <string.h>

#define MAX_BLK_CNT		4		/* max # blocks per call in TestTwofish */
static int
TestTwofish( int mode, int keySize ) /* keySize must be 128, 192, or 256 */
{								/* return 0 iff test passes */
	keyInstance    ki;			/* key information, including tables */
	cipherInstance ci;			/* keeps mode (ECB, CBC) and IV */
	BYTE  plainText[MAX_BLK_CNT*(BLOCK_SIZE/8)];
	BYTE cipherText[MAX_BLK_CNT*(BLOCK_SIZE/8)];
	BYTE decryptOut[MAX_BLK_CNT*(BLOCK_SIZE/8)];
	BYTE iv[BLOCK_SIZE/8];
	int  i,byteCnt;

	if (makeKey(&ki,DIR_ENCRYPT,keySize,NULL) != TRUE)
		return 1;				/* 'dummy' setup for a 128-bit key */
	if (cipherInit(&ci,mode,NULL) != TRUE)
		return 1;				/* 'dummy' setup for cipher */
	
	for (i=0;i<keySize/32;i++)	/* select key bits */
		ki.key32[i]=0x10003 * rand();
	reKey(&ki);					/* run the key schedule */

	if (mode != MODE_ECB)		/* set up random iv (if needed)*/
		{
		for (i=0;i<sizeof(iv);i++)
			iv[i]=(BYTE) rand();
		memcpy(ci.iv32,iv,sizeof(ci.iv32));	/* copy the IV to ci */
		}

	/* select number of bytes to encrypt (multiple of block) */
	/* e.g., byteCnt = 16, 32, 48, 64 */
	byteCnt = (BLOCK_SIZE/8) * (1 + (rand() % MAX_BLK_CNT));

	for (i=0;i<byteCnt;i++)		/* generate test data */
		plainText[i]=(BYTE) rand();
	
	/* encrypt the bytes */
	if (blockEncrypt(&ci,&ki, plainText,byteCnt*8,cipherText) != byteCnt*8)
		return 1;

	/* decrypt the bytes */
	if (mode != MODE_ECB)		/* first re-init the IV (if needed) */
		memcpy(ci.iv32,iv,sizeof(ci.iv32));

	if (blockDecrypt(&ci,&ki,cipherText,byteCnt*8,decryptOut) != byteCnt*8)
		return 1;				
	
	/* make sure the decrypt output matches original plaintext */
	if (memcmp(plainText,decryptOut,byteCnt))
		return 1;		

	return 0;					/* tests passed! */
}


int
main( void )
{
	srand( (unsigned int) time( NULL ) );	/* randomize */
	
	for ( int keySize = 128; keySize <= 256; keySize += 64 )
		{
		for ( int testCnt = 0; testCnt < 10; ++testCnt )
			{
			if ( TestTwofish( MODE_ECB, keySize ) )
				{
				printf( "ECB Failure at keySize = %d\n", keySize );
				return EXIT_FAILURE;
				}
			
			if ( TestTwofish( MODE_CBC, keySize ) )
				{
				printf( "CBC Failure at keySize = %d\n", keySize );
				return EXIT_FAILURE;
				}
			}
		}
	
	printf( "Tests passed\n" );
	return EXIT_SUCCESS;
}
#endif /* TEST_2FISH */


/***************************************************************************
	TABLE.H	-- Tables, macros, constants for Twofish S-boxes and MDS matrix

	Submitters:
		Bruce Schneier, Counterpane Systems
		Doug Whiting,	Hi/fn
		John Kelsey,	Counterpane Systems
		Chris Hall,		Counterpane Systems
		David Wagner,	UC Berkeley
			
	Code Author:		Doug Whiting,	Hi/fn
		
	Version  1.00		April 1998
		
	Copyright 1998, Hi/fn and Counterpane Systems.  All rights reserved.
		
	Notes:
		*	Tab size is set to 4 characters in this file
		*	These definitions should be used in optimized and unoptimized
			versions to insure consistency.

***************************************************************************/

/* for computing subkeys */
#define	SK_STEP			0x02020202u
#define	SK_BUMP			0x01010101u
#define	SK_ROTL			9

/* Reed-Solomon code parameters: (12,8) reversible code
	g(x) = x**4 + (a + 1/a) x**3 + a x**2 + (a + 1/a) x + 1
   where a = primitive root of field generator 0x14D */
#define	RS_GF_FDBK		0x14D		/* field generator */
#define	RS_rem(x)		\
	{ BYTE  b  = (BYTE) (x >> 24);											 \
	  DWORD g2 = ((b << 1) ^ ((b & 0x80) ? RS_GF_FDBK : 0 )) & 0xFF;		 \
	  DWORD g3 = ((b >> 1) & 0x7F) ^ ((b & 1) ? RS_GF_FDBK >> 1 : 0 ) ^ g2 ; \
	  x = (x << 8) ^ (g3 << 24) ^ (g2 << 16) ^ (g3 << 8) ^ b;				 \
	}

/*	Macros for the MDS matrix
*	The MDS matrix is (using primitive polynomial 169):
*      01  EF  5B  5B
*      5B  EF  EF  01
*      EF  5B  01  EF
*      EF  01  EF  5B
*----------------------------------------------------------------
* More statistical properties of this matrix (from MDS.EXE output):
*
* Min Hamming weight (one byte difference) =  8. Max=26.  Total =  1020.
* Prob[8]:      7    23    42    20    52    95    88    94   121   128    91
*             102    76    41    24     8     4     1     3     0     0     0
* Runs[8]:      2     4     5     6     7     8     9    11
* MSBs[8]:      1     4    15     8    18    38    40    43
* HW= 8: 05040705 0A080E0A 14101C14 28203828 50407050 01499101 A080E0A0 
* HW= 9: 04050707 080A0E0E 10141C1C 20283838 40507070 80A0E0E0 C6432020 07070504 
*        0E0E0A08 1C1C1410 38382820 70705040 E0E0A080 202043C6 05070407 0A0E080E 
*        141C101C 28382038 50704070 A0E080E0 4320C620 02924B02 089A4508 
* Min Hamming weight (two byte difference) =  3. Max=28.  Total = 390150.
* Prob[3]:      7    18    55   149   270   914  2185  5761 11363 20719 32079
*           43492 51612 53851 52098 42015 31117 20854 11538  6223  2492  1033
* MDS OK, ROR:   6+  7+  8+  9+ 10+ 11+ 12+ 13+ 14+ 15+ 16+
*               17+ 18+ 19+ 20+ 21+ 22+ 23+ 24+ 25+ 26+
*/
#define	MDS_GF_FDBK		0x169	/* primitive polynomial for GF(256)*/
#define	LFSR1(x) ( ((x) >> 1)  ^ (((x) & 0x01) ?   MDS_GF_FDBK/2 : 0))
#define	LFSR2(x) ( ((x) >> 2)  ^ (((x) & 0x02) ?   MDS_GF_FDBK/2 : 0)  \
							   ^ (((x) & 0x01) ?   MDS_GF_FDBK/4 : 0))

#define	Mx_1(x) ((DWORD)  (x))		/* force result to dword so << will work */
#define	Mx_X(x) ((DWORD) ((x) ^            LFSR2(x)))	/* 5B */
#define	Mx_Y(x) ((DWORD) ((x) ^ LFSR1(x) ^ LFSR2(x)))	/* EF */

#define	M00		Mul_1
#define	M01		Mul_Y
#define	M02		Mul_X
#define	M03		Mul_X

#define	M10		Mul_X
#define	M11		Mul_Y
#define	M12		Mul_Y
#define	M13		Mul_1

#define	M20		Mul_Y
#define	M21		Mul_X
#define	M22		Mul_1
#define	M23		Mul_Y

#define	M30		Mul_Y
#define	M31		Mul_1
#define	M32		Mul_Y
#define	M33		Mul_X

#define	Mul_1	Mx_1
#define	Mul_X	Mx_X
#define	Mul_Y	Mx_Y

/*	Define the fixed p0/p1 permutations used in keyed S-box lookup.  
	By changing the following constant definitions for P_ij, the S-boxes will
	automatically get changed in all the Twofish source code. Note that P_i0 is
	the "outermost" 8x8 permutation applied.  See the f32() function to see
	how these constants are to be  used.
*/
#define	P_00	1					/* "outermost" permutation */
#define	P_01	0
#define	P_02	0
#define	P_03	(P_01^1)			/* "extend" to larger key sizes */
#define	P_04	1

#define	P_10	0
#define	P_11	0
#define	P_12	1
#define	P_13	(P_11^1)
#define	P_14	0

#define	P_20	1
#define	P_21	1
#define	P_22	0
#define	P_23	(P_21^1)
#define	P_24	0

#define	P_30	0
#define	P_31	1
#define	P_32	1
#define	P_33	(P_31^1)
#define	P_34	1

#define	p8(N)	P8x8[ P_##N ]		/* some syntax shorthand */

/* fixed 8x8 permutation S-boxes */

/***********************************************************************
*  07:07:14  05/30/98  [4x4]  TestCnt=256. keySize=128. CRC=4BD14D9E.
* maxKeyed:  dpMax = 18. lpMax =100. fixPt =  8. skXor =  0. skDup =  6. 
* log2(dpMax[ 6..18])=   --- 15.42  1.33  0.89  4.05  7.98 12.05
* log2(lpMax[ 7..12])=  9.32  1.01  1.16  4.23  8.02 12.45
* log2(fixPt[ 0.. 8])=  1.44  1.44  2.44  4.06  6.01  8.21 11.07 14.09 17.00
* log2(skXor[ 0.. 0])
* log2(skDup[ 0.. 6])=   ---  2.37  0.44  3.94  8.36 13.04 17.99
***********************************************************************/

static const BYTE
P8x8[2][256] ALIGN_ME = {

/*  p0:   */
/*  dpMax      = 10.  lpMax      = 64.  cycleCnt=   1  1  1  0.         */
/* 817D6F320B59ECA4.ECB81235F4A6709D.BA5E6D90C8F32471.D7F4126E9B3085CA. */
/* Karnaugh maps:
*  0111 0001 0011 1010. 0001 1001 1100 1111. 1001 1110 0011 1110. 1101 0101 1111 1001. 
*  0101 1111 1100 0100. 1011 0101 0010 0000. 0101 1000 1100 0101. 1000 0111 0011 0010. 
*  0000 1001 1110 1101. 1011 1000 1010 0011. 0011 1001 0101 0000. 0100 0010 0101 1011. 
*  0111 0100 0001 0110. 1000 1011 1110 1001. 0011 0011 1001 1101. 1101 0101 0000 1100. 
*/
	{
	0xA9, 0x67, 0xB3, 0xE8, 0x04, 0xFD, 0xA3, 0x76, 
	0x9A, 0x92, 0x80, 0x78, 0xE4, 0xDD, 0xD1, 0x38, 
	0x0D, 0xC6, 0x35, 0x98, 0x18, 0xF7, 0xEC, 0x6C, 
	0x43, 0x75, 0x37, 0x26, 0xFA, 0x13, 0x94, 0x48, 
	0xF2, 0xD0, 0x8B, 0x30, 0x84, 0x54, 0xDF, 0x23, 
	0x19, 0x5B, 0x3D, 0x59, 0xF3, 0xAE, 0xA2, 0x82, 
	0x63, 0x01, 0x83, 0x2E, 0xD9, 0x51, 0x9B, 0x7C, 
	0xA6, 0xEB, 0xA5, 0xBE, 0x16, 0x0C, 0xE3, 0x61, 
	0xC0, 0x8C, 0x3A, 0xF5, 0x73, 0x2C, 0x25, 0x0B, 
	0xBB, 0x4E, 0x89, 0x6B, 0x53, 0x6A, 0xB4, 0xF1, 
	0xE1, 0xE6, 0xBD, 0x45, 0xE2, 0xF4, 0xB6, 0x66, 
	0xCC, 0x95, 0x03, 0x56, 0xD4, 0x1C, 0x1E, 0xD7, 
	0xFB, 0xC3, 0x8E, 0xB5, 0xE9, 0xCF, 0xBF, 0xBA, 
	0xEA, 0x77, 0x39, 0xAF, 0x33, 0xC9, 0x62, 0x71, 
	0x81, 0x79, 0x09, 0xAD, 0x24, 0xCD, 0xF9, 0xD8, 
	0xE5, 0xC5, 0xB9, 0x4D, 0x44, 0x08, 0x86, 0xE7, 
	0xA1, 0x1D, 0xAA, 0xED, 0x06, 0x70, 0xB2, 0xD2, 
	0x41, 0x7B, 0xA0, 0x11, 0x31, 0xC2, 0x27, 0x90, 
	0x20, 0xF6, 0x60, 0xFF, 0x96, 0x5C, 0xB1, 0xAB, 
	0x9E, 0x9C, 0x52, 0x1B, 0x5F, 0x93, 0x0A, 0xEF, 
	0x91, 0x85, 0x49, 0xEE, 0x2D, 0x4F, 0x8F, 0x3B, 
	0x47, 0x87, 0x6D, 0x46, 0xD6, 0x3E, 0x69, 0x64, 
	0x2A, 0xCE, 0xCB, 0x2F, 0xFC, 0x97, 0x05, 0x7A, 
	0xAC, 0x7F, 0xD5, 0x1A, 0x4B, 0x0E, 0xA7, 0x5A, 
	0x28, 0x14, 0x3F, 0x29, 0x88, 0x3C, 0x4C, 0x02, 
	0xB8, 0xDA, 0xB0, 0x17, 0x55, 0x1F, 0x8A, 0x7D, 
	0x57, 0xC7, 0x8D, 0x74, 0xB7, 0xC4, 0x9F, 0x72, 
	0x7E, 0x15, 0x22, 0x12, 0x58, 0x07, 0x99, 0x34, 
	0x6E, 0x50, 0xDE, 0x68, 0x65, 0xBC, 0xDB, 0xF8, 
	0xC8, 0xA8, 0x2B, 0x40, 0xDC, 0xFE, 0x32, 0xA4, 
	0xCA, 0x10, 0x21, 0xF0, 0xD3, 0x5D, 0x0F, 0x00, 
	0x6F, 0x9D, 0x36, 0x42, 0x4A, 0x5E, 0xC1, 0xE0
	},
	
/*  p1:   */
/*  dpMax      = 10.  lpMax      = 64.  cycleCnt=   2  0  0  1.         */
/* 28BDF76E31940AC5.1E2B4C376DA5F908.4C75169A0ED82B3F.B951C3DE647F208A. */
/* Karnaugh maps:
*  0011 1001 0010 0111. 1010 0111 0100 0110. 0011 0001 1111 0100. 1111 1000 0001 1100. 
*  1100 1111 1111 1010. 0011 0011 1110 0100. 1001 0110 0100 0011. 0101 0110 1011 1011. 
*  0010 0100 0011 0101. 1100 1000 1000 1110. 0111 1111 0010 0110. 0000 1010 0000 0011. 
*  1101 1000 0010 0001. 0110 1001 1110 0101. 0001 0100 0101 0111. 0011 1011 1111 0010. 
*/
	{
	0x75, 0xF3, 0xC6, 0xF4, 0xDB, 0x7B, 0xFB, 0xC8, 
	0x4A, 0xD3, 0xE6, 0x6B, 0x45, 0x7D, 0xE8, 0x4B, 
	0xD6, 0x32, 0xD8, 0xFD, 0x37, 0x71, 0xF1, 0xE1, 
	0x30, 0x0F, 0xF8, 0x1B, 0x87, 0xFA, 0x06, 0x3F, 
	0x5E, 0xBA, 0xAE, 0x5B, 0x8A, 0x00, 0xBC, 0x9D, 
	0x6D, 0xC1, 0xB1, 0x0E, 0x80, 0x5D, 0xD2, 0xD5, 
	0xA0, 0x84, 0x07, 0x14, 0xB5, 0x90, 0x2C, 0xA3, 
	0xB2, 0x73, 0x4C, 0x54, 0x92, 0x74, 0x36, 0x51, 
	0x38, 0xB0, 0xBD, 0x5A, 0xFC, 0x60, 0x62, 0x96, 
	0x6C, 0x42, 0xF7, 0x10, 0x7C, 0x28, 0x27, 0x8C, 
	0x13, 0x95, 0x9C, 0xC7, 0x24, 0x46, 0x3B, 0x70, 
	0xCA, 0xE3, 0x85, 0xCB, 0x11, 0xD0, 0x93, 0xB8, 
	0xA6, 0x83, 0x20, 0xFF, 0x9F, 0x77, 0xC3, 0xCC, 
	0x03, 0x6F, 0x08, 0xBF, 0x40, 0xE7, 0x2B, 0xE2, 
	0x79, 0x0C, 0xAA, 0x82, 0x41, 0x3A, 0xEA, 0xB9, 
	0xE4, 0x9A, 0xA4, 0x97, 0x7E, 0xDA, 0x7A, 0x17, 
	0x66, 0x94, 0xA1, 0x1D, 0x3D, 0xF0, 0xDE, 0xB3, 
	0x0B, 0x72, 0xA7, 0x1C, 0xEF, 0xD1, 0x53, 0x3E, 
	0x8F, 0x33, 0x26, 0x5F, 0xEC, 0x76, 0x2A, 0x49, 
	0x81, 0x88, 0xEE, 0x21, 0xC4, 0x1A, 0xEB, 0xD9, 
	0xC5, 0x39, 0x99, 0xCD, 0xAD, 0x31, 0x8B, 0x01, 
	0x18, 0x23, 0xDD, 0x1F, 0x4E, 0x2D, 0xF9, 0x48, 
	0x4F, 0xF2, 0x65, 0x8E, 0x78, 0x5C, 0x58, 0x19, 
	0x8D, 0xE5, 0x98, 0x57, 0x67, 0x7F, 0x05, 0x64, 
	0xAF, 0x63, 0xB6, 0xFE, 0xF5, 0xB7, 0x3C, 0xA5, 
	0xCE, 0xE9, 0x68, 0x44, 0xE0, 0x4D, 0x43, 0x69, 
	0x29, 0x2E, 0xAC, 0x15, 0x59, 0xA8, 0x0A, 0x9E, 
	0x6E, 0x47, 0xDF, 0x34, 0x35, 0x6A, 0xCF, 0xDC, 
	0x22, 0xC9, 0xC0, 0x9B, 0x89, 0xD4, 0xED, 0xAB, 
	0x12, 0xA2, 0x0D, 0x52, 0xBB, 0x02, 0x2F, 0xA9, 
	0xD7, 0x61, 0x1E, 0xB4, 0x50, 0x04, 0xF6, 0xC2, 
	0x16, 0x25, 0x86, 0x56, 0x55, 0x09, 0xBE, 0x91
	}
	};


/***************************************************************************
	TWOFISH2.C	-- Optimized C API calls for TWOFISH AES submission

	Submitters:
		Bruce Schneier, Counterpane Systems
		Doug Whiting,	Hi/fn
		John Kelsey,	Counterpane Systems
		Chris Hall,		Counterpane Systems
		David Wagner,	UC Berkeley
			
	Code Author:		Doug Whiting,	Hi/fn
		
	Version  1.00		April 1998
		
	Copyright 1998, Hi/fn and Counterpane Systems.  All rights reserved.
		
	Notes:
		*	Optimized version
		*	Tab size is set to 4 characters in this file

***************************************************************************/

//#include	<memory.h>
//#include	<assert.h>

#if   defined(min_key)  && !defined(MIN_KEY)
#define	MIN_KEY		1			/* toupper() */
#elif defined(part_key) && !defined(PART_KEY)
#define	PART_KEY	1
#elif defined(zero_key) && !defined(ZERO_KEY)
#define	ZERO_KEY	1
#endif

#ifdef USE_ASM
extern	int	useAsm;				/* ok to use ASM code? */

typedef	int cdecl CipherProc
   (cipherInstance * cipher, keyInstance * key, BYTE * input, int inputLen, BYTE * outBuffer );
typedef int	cdecl KeySetupProc( keyInstance * key );

extern CipherProc	* blockEncrypt_86;	/* ptr to ASM functions */
extern CipherProc	* blockDecrypt_86;
extern KeySetupProc	* reKey_86;
extern DWORD		cdecl TwofishAsmCodeSize( void );
#endif	/* USE_ASM */

/*
** prototypes
*/
//static int		ParseHexDword( int bits, const char * srcTxt, DWORD * d, char * dstTxt );
static DWORD	RS_MDS_Encode( DWORD k0, DWORD k1 );
static void 	BuildMDS( void );
static void		ReverseRoundSubkeys( keyInstance * key, BYTE newDir );
static void		Xor256( DWORD * dst, const DWORD * src, BYTE b );

//static DWORD TwofishCodeSize( void );


/*
+*****************************************************************************
*			Constants/Macros/Tables
-****************************************************************************/

static fullSbox		MDStab;					/* not actually const: Initialized ONE time */
// rer sez: "const" means "const", dangit. Anything else, isn't.

static bool			bIsMDSInitialized = false;	/* is MDStab initialized yet? */

#define	BIG_TAB		0

#if BIG_TAB
static BYTE		bigTab[4][256][256];	/* pre-computed S-box */
#endif

/* number of rounds for various key sizes:  128, 192, 256 */
/* (ignored for now in optimized code!) */
static const int	numRounds[4]= { 0, ROUNDS_128, ROUNDS_192, ROUNDS_256 };

#if REENTRANT
#define		_sBox_	 key->sBox8x32
#else
static		fullSbox _sBox_;		/* permuted MDStab based on keys */
#endif  // REENTRANT

#define _sBox8_(N) ( ((const BYTE *) _sBox_) + (N) * 256 )

/*------- see what level of S-box precomputation we need to do -----*/
#if   defined( ZERO_KEY )
#define	MOD_STRING	"(Zero S-box keying)"
#define	Fe32_128(x,R)	\
	(	MDStab[0][p8(01)[p8(02)[_b(x,R  )]^b0(SKEY[1])]^b0(SKEY[0])] ^	\
		MDStab[1][p8(11)[p8(12)[_b(x,R+1)]^b1(SKEY[1])]^b1(SKEY[0])] ^	\
		MDStab[2][p8(21)[p8(22)[_b(x,R+2)]^b2(SKEY[1])]^b2(SKEY[0])] ^	\
		MDStab[3][p8(31)[p8(32)[_b(x,R+3)]^b3(SKEY[1])]^b3(SKEY[0])] )
#define	Fe32_192(x,R)	\
	(	MDStab[0][p8(01)[p8(02)[p8(03)[_b(x,R  )]^b0(SKEY[2])]^b0(SKEY[1])]^b0(SKEY[0])] ^ \
		MDStab[1][p8(11)[p8(12)[p8(13)[_b(x,R+1)]^b1(SKEY[2])]^b1(SKEY[1])]^b1(SKEY[0])] ^ \
		MDStab[2][p8(21)[p8(22)[p8(23)[_b(x,R+2)]^b2(SKEY[2])]^b2(SKEY[1])]^b2(SKEY[0])] ^ \
		MDStab[3][p8(31)[p8(32)[p8(33)[_b(x,R+3)]^b3(SKEY[2])]^b3(SKEY[1])]^b3(SKEY[0])] )
#define	Fe32_256(x,R)	\
	(	MDStab[0][p8(01)[p8(02)[p8(03)[p8(04)[_b(x,R  )]^b0(SKEY[3])]^b0(SKEY[2])]^b0(SKEY[1])]^b0(SKEY[0])] ^ \
		MDStab[1][p8(11)[p8(12)[p8(13)[p8(14)[_b(x,R+1)]^b1(SKEY[3])]^b1(SKEY[2])]^b1(SKEY[1])]^b1(SKEY[0])] ^ \
		MDStab[2][p8(21)[p8(22)[p8(23)[p8(24)[_b(x,R+2)]^b2(SKEY[3])]^b2(SKEY[2])]^b2(SKEY[1])]^b2(SKEY[0])] ^ \
		MDStab[3][p8(31)[p8(32)[p8(33)[p8(34)[_b(x,R+3)]^b3(SKEY[3])]^b3(SKEY[2])]^b3(SKEY[1])]^b3(SKEY[0])] )

#define	GetSboxKey	DWORD SKEY[4];	/* local copy */ \
					memcpy( SKEY, key->sboxKeys, sizeof(SKEY) );
/*----------------------------------------------------------------*/
#elif defined( MIN_KEY )
#define	MOD_STRING	"(Minimal keying)"
#define	Fe32_(x,R)(MDStab[0][p8(01)[_sBox8_(0)[_b(x,R  )]] ^ b0(SKEY0)] ^ \
				   MDStab[1][p8(11)[_sBox8_(1)[_b(x,R+1)]] ^ b1(SKEY0)] ^ \
				   MDStab[2][p8(21)[_sBox8_(2)[_b(x,R+2)]] ^ b2(SKEY0)] ^ \
				   MDStab[3][p8(31)[_sBox8_(3)[_b(x,R+3)]] ^ b3(SKEY0)])
#define sbSet(N,i,J,v) { _sBox8_(N)[i+J] = v; }
#define	GetSboxKey	DWORD SKEY0	= key->sboxKeys[0]		/* local copy */
/*----------------------------------------------------------------*/
#elif defined( PART_KEY )	
#define	MOD_STRING	"(Partial keying)"
#define	Fe32_(x,R)(MDStab[0][_sBox8_(0)[_b(x,R  )]] ^ \
				   MDStab[1][_sBox8_(1)[_b(x,R+1)]] ^ \
				   MDStab[2][_sBox8_(2)[_b(x,R+2)]] ^ \
				   MDStab[3][_sBox8_(3)[_b(x,R+3)]])
#define sbSet(N,i,J,v) { _sBox8_(N)[i+J] = v; }
#define	GetSboxKey	
/*----------------------------------------------------------------*/
#else	/* default is FULL_KEY */
#ifndef FULL_KEY
#define	FULL_KEY	1
#endif

#if BIG_TAB
#define	TAB_STR		" (Big table)"
#else
#define	TAB_STR
#endif

#ifdef COMPILE_KEY
#define	MOD_STRING	"(Compiled subkeys)" TAB_STR
#else
#define	MOD_STRING	"(Full keying)" TAB_STR
#endif

/* Fe32_ does a full S-box + MDS lookup.  Need to #define _sBox_ before use.
   Note that we "interleave" 0,1, and 2,3 to avoid cache bank collisions
   in optimized assembly language.
*/
#define	Fe32_(x,R) (_sBox_[0][2*_b(x,R  )] ^ _sBox_[0][2*_b(x,R+1)+1] ^	\
				    _sBox_[2][2*_b(x,R+2)] ^ _sBox_[2][2*_b(x,R+3)+1])

		/* set a single S-box value, given the input byte */
#if 0
# define sbSet(N,i,J,v) { _sBox_[N&2][2*i+(N&1)+2*J]=MDStab[N][v]; }
#else
static inline void
sbSet( uchar N, uint i, uchar J, DWORD v )
{
	// the macro definition above yields an analyzer warning for all 'i' > 128
	// (and that macro is called with i = 0, 2, 4 ... 254).
	// _sBox_ is declared as: DWORD[ 4 ][ 256 ]
	// so when 'i' is, e.g., 200 and N and J are 0 then the macro tries to access
	//  _sBox[0][ 400 ]
	// ... where the 2nd index is clearly out of bounds.
	// What they're really doing, of course, is treating _sBox_ as if it were
	// declared as:  DWORD[ 2 ][ 512 ]
	// 
	// So here is a rewritten version that does the same thing, but legally.
	uint sb2 = 2 * i + (N & 1) + 2 * J;
	uchar N2 = N & 2;	// 1st index 0 or 2
	if ( sb2 >= 256 )	// but if we're exceeding that row...
		{
		++N2;			// adjust 1st & 2nd index
		sb2 -= 256;		// into legal ranges
		}
//	assert( sb2 < 256 );
	_sBox_[ N2 ][ sb2 ] = MDStab[ N ][ v ];
}
#endif  // 0

#define	GetSboxKey	
#endif  // ZERO_KEY / MIN_KEY / PART_KEY / FULL_KEY

//static const		char moduleDescription[]	= "Optimized C ";
//static const		char modeString[]			= MOD_STRING;


/* macro(s) for debugging help */
#define		CHECK_TABLE		0		/* nonzero --> compare against "slow" table */
#define		VALIDATE_PARMS	0		/* disable for full speed */

#ifdef DEBUG_ENCRYPTION	/* keep these macros common so they are same for both versions */
static const int debugCompile	=	1;
extern  int	debug;
extern  void DebugIO( const char * s );	/* display the debug output */

#define DebugDump(x,s,R,XOR,doRot,showT,needBswap)	\
	{ if (debug) _Dump( x, s, R, XOR, doRot, showT, needBswap, t0, t1 ); }

#define	DebugDumpKey(key) { if (debug) _DumpKey( key ); }

#define	IV_ROUND	-100


static void
_Dump( const void * p, const char * s, int R, int XOR, int doRot, int showT, int needBswap,
		   DWORD t0, DWORD t1 )
{
	char line[ 512 ];	/* build output here */
	int  i,n;
	DWORD q[4];
	
	if ( R == IV_ROUND )
		snprintf( line, sizeof line, "%sIV:    ", s );
	else
		snprintf( line, sizeof line, "%sR[%2d]: ", s, R );
	for ( n = 0; line[ n ]; ++n)
		;
	
	for ( i = 0; i < 4; ++i )
		{
		q[i] = ( (const DWORD *) p)[ i ^ (XOR) ];
		if ( needBswap )
			q[i] = Bswap( q[i] );
		}
	
	snprintf( line + n, sizeof line - n,
			"x= %08lX  %08lX  %08lX  %08lX.",
			ROR( q[0], doRot * ( R    ) / 2 ),
			ROL( q[1], doRot * ( R    ) / 2 ),
			ROR( q[2], doRot * ( R + 1) / 2 ),
			ROL( q[3], doRot * ( R + 1) / 2 ) );
	
	for ( ; line[n]; ++n )
		;

	if ( showT )
		snprintf( line + n, sizeof line - n, "    t0=%08lX. t1=%08lX.", t0, t1 );
	for ( ; line[n]; ++n)
		;
	
	snprintf( line + n, sizeof line - n, "\n" );
	DebugIO( line );
}


static void
_DumpKey( const keyInstance * key )
{
	char	line[ 512 ];
	int		i;
	int		k64Cnt = ( key->keyLen + 63 ) / 64;		/* round up to next multiple of 64 bits */
	int		subkeyCnt = ROUND_SUBKEYS + 2 * key->numRounds;
	
	sprintf( line, ";\n;makeKey:   Input key            -->  S-box key     [%s]\n",
		   ( key->direction == DIR_ENCRYPT ) ? "Encrypt" : "Decrypt" );
	DebugIO( line );
	
	for ( i = 0; i < k64Cnt ; ++i )	/* display in RS format */
		{
		sprintf( line, ";%12s %08lX %08lX  -->  %08lX\n", "",
			   key->key32[ 2 * i + 1 ], key->key32[ 2 * i ], key->sboxKeys[ k64Cnt - 1 - i ] );
		DebugIO( line );
		}
	sprintf( line, ";%11sSubkeys\n", "" );
	DebugIO( line );
	for ( i = 0; i < subkeyCnt / 2; ++i )
		{
		sprintf( line, ";%12s %08lX %08lX%s\n", "", key->subKeys[ 2 * i ], key->subKeys[ 2 * i + 1 ],
			  ( 2 * i == INPUT_WHITEN  ) ? "   Input whiten" :
			  ( 2 * i == OUTPUT_WHITEN ) ? "  Output whiten" :
		      ( 2 * i == ROUND_SUBKEYS ) ? "  Round subkeys" : "" );
		DebugIO( line );
		}
	DebugIO( ";\n" );
}
#else
//static const int debugCompile	=	0;
#define DebugDump(x, s, R, XOR, doRot, showT, needBswap )
#define	DebugDumpKey( key )
#endif

//#include	"debug.h"				/* debug display macros */

/* end of debug macros */

#if 0
extern DWORD Here( DWORD x );			/* return caller's address! */
static DWORD TwofishCodeStart( void );

static DWORD
TwofishCodeStart( void )
{
	return Here(0);
}
#endif


/*
+*****************************************************************************
*
* Function Name:	TableOp
*
* Function:			Handle table use checking
*
* Arguments:		op	=	what to do	(see TAB_* defns in AES.H)
*
* Return:			TRUE --> done (for TAB_QUERY)		
*
* Notes: This routine is for use in generating the tables KAT file.
*		 For this optimized version, we don't actually track table usage,
*		 since it would make the macros incredibly ugly.  Instead we just
*		 run for a fixed number of queries and then say we're done.
*
-****************************************************************************/
#if 0
int
TableOp( int op )
{
	static int queryCnt = 0;
	
	switch ( op )
		{
		case TAB_DISABLE:
			break;
		case TAB_ENABLE:
			break;
		case TAB_RESET:
			queryCnt=0;
			break;
		case TAB_QUERY:
			++queryCnt;
			if ( queryCnt < TAB_MIN_QUERY )
				return FALSE;
		}
	return TRUE;
}
#endif


#if 0
/*
+*****************************************************************************
*
* Function Name:	ParseHexDword
*
* Function:			Parse ASCII hex nibbles and fill in key/iv dwords
*
* Arguments:		bit			=	# bits to read
*					srcTxt		=	ASCII source
*					d			=	ptr to dwords to fill in
*					dstTxt		=	where to make a copy of ASCII source
*									(NULL ok)
*
* Return:			Zero if no error.  Nonzero --> invalid hex or length
*
* Notes:  Note that the parameter d is a DWORD array, not a byte array.
*	This routine is coded to work both for little-endian and big-endian
*	architectures.  The character stream is interpreted as a LITTLE-ENDIAN
*	byte stream, since that is how the Pentium works, but the conversion
*	happens automatically below. 
*
-****************************************************************************/
int
ParseHexDword( int bits, const char * srcTxt, DWORD * d, char * dstTxt )
{
	int i;
	DWORD b;

	union	/* make sure LittleEndian is defined correctly */
		{
		BYTE  b[4];
		DWORD d[1];
		} v;
	
	v.d[0] = 1;
	if ( v.b[ 0 ^ ADDR_XOR ] != 1 )
		return BAD_ENDIAN;		/* make sure compile-time switch is set ok */

#if VALIDATE_PARMS
  #if ALIGN32
	if (((int)d) & 3)
		return BAD_ALIGN32;
  #endif
#endif

	for ( i = 0; i * 32 < bits; ++i )
		d[i] = 0;							/* first, zero the field */
	
	for ( i = 0; i * 4 < bits; ++i )		/* parse one nibble at a time */
		{									/* case out the hexadecimal characters */
		char c = srcTxt[i];
		
		if ( dstTxt )
			dstTxt[i] = c;
		
		if ( isdigit(c) )
			b = c - '0';
		else
		if ( isxdigit(c) )
			b = toupper(c) - 'A' + 10;
		else
			return BAD_KEY_MAT;	/* invalid hex character */
		
		/* works for big and little endian! */
		d[ i/8 ] |= b << ( 4 * ( (i^1) & 7 ) );		
		}
	
	return 0;					/* no error */
}
#endif // 0

#if CHECK_TABLE
/*
+*****************************************************************************
*
* Function Name:	f32
*
* Function:			Run four bytes through keyed S-boxes and apply MDS matrix
*
* Arguments:		x			=	input to f function
*					k32			=	pointer to key dwords
*					keyLen		=	total key length (k32 --> keyLey/2 bits)
*
* Return:			The output of the keyed permutation applied to x.
*
* Notes:
*	This function is a keyed 32-bit permutation.  It is the major building
*	block for the Twofish round function, including the four keyed 8x8 
*	permutations and the 4x4 MDS matrix multiply.  This function is used
*	both for generating round subkeys and within the round function on the
*	block being encrypted.  
*
*	This version is fairly slow and pedagogical, although a smartcard would
*	probably perform the operation exactly this way in firmware.   For
*	ultimate performance, the entire operation can be completed with four
*	lookups into four 256x32-bit tables, with three dword xors.
*
*	The MDS matrix is defined in TABLE.H.  To multiply by Mij, just use the
*	macro Mij(x).
*
-****************************************************************************/
static DWORD
f32( DWORD x, const DWORD * k32, int keyLen )
{
	BYTE  b[4];
	
	/* Run each byte thru 8x8 S-boxes, xoring with key byte at each stage. */
	/* Note that each byte goes through a different combination of S-boxes.*/

	*((DWORD *) b) = Bswap( x );	/* make b[0] = LSB, b[3] = MSB */
	switch ( ((keyLen + 63)/64) & 3 )
		{
		case 0:		/* 256 bits of key */
			b[0] = p8(04)[b[0]] ^ b0(k32[3]);
			b[1] = p8(14)[b[1]] ^ b1(k32[3]);
			b[2] = p8(24)[b[2]] ^ b2(k32[3]);
			b[3] = p8(34)[b[3]] ^ b3(k32[3]);
			/* fall thru, having pre-processed b[0]..b[3] with k32[3] */
			FALLTHRU_OK;
		case 3:		/* 192 bits of key */
			b[0] = p8(03)[b[0]] ^ b0(k32[2]);
			b[1] = p8(13)[b[1]] ^ b1(k32[2]);
			b[2] = p8(23)[b[2]] ^ b2(k32[2]);
			b[3] = p8(33)[b[3]] ^ b3(k32[2]);
			/* fall thru, having pre-processed b[0]..b[3] with k32[2] */
			FALLTHRU_OK;
		case 2:		/* 128 bits of key */
			b[0] = p8(00)[p8(01)[p8(02)[b[0]] ^ b0(k32[1])] ^ b0(k32[0])];
			b[1] = p8(10)[p8(11)[p8(12)[b[1]] ^ b1(k32[1])] ^ b1(k32[0])];
			b[2] = p8(20)[p8(21)[p8(22)[b[2]] ^ b2(k32[1])] ^ b2(k32[0])];
			b[3] = p8(30)[p8(31)[p8(32)[b[3]] ^ b3(k32[1])] ^ b3(k32[0])];
		}

	/* Now perform the MDS matrix multiply inline. */
	return	((M00(b[0]) ^ M01(b[1]) ^ M02(b[2]) ^ M03(b[3]))	  ) ^
			((M10(b[0]) ^ M11(b[1]) ^ M12(b[2]) ^ M13(b[3])) <<  8) ^
			((M20(b[0]) ^ M21(b[1]) ^ M22(b[2]) ^ M23(b[3])) << 16) ^
			((M30(b[0]) ^ M31(b[1]) ^ M32(b[2]) ^ M33(b[3])) << 24) ;
}
#endif	/* CHECK_TABLE */


/*
+*****************************************************************************
*
* Function Name:	RS_MDS_encode
*
* Function:			Use (12,8) Reed-Solomon code over GF(256) to produce
*					a key S-box dword from two key material dwords.
*
* Arguments:		k0	=	1st dword
*					k1	=	2nd dword
*
* Return:			Remainder polynomial generated using RS code
*
* Notes:
*	Since this computation is done only once per reKey per 64 bits of key,
*	the performance impact of this routine is imperceptible. The RS code
*	chosen has "simple" coefficients to allow smartcard/hardware implementation
*	without lookup tables.
*
-****************************************************************************/
DWORD
RS_MDS_Encode( DWORD k0, DWORD k1 )
{
	int i,j;
	DWORD r;

	for ( i = r = 0; i < 2; ++i )
		{
		r ^= (i) ? k0 : k1;			/* merge in 32 more key bits */
		for ( j = 0; j < 4; ++j )	/* shift one byte at a time */
			RS_rem( r );
		}
	return r;
}


/*
+*****************************************************************************
*
* Function Name:	BuildMDS
*
* Function:			Initialize the MDStab array
*
* Arguments:		None.
*
* Return:			None.
*
* Notes:
*	Here we precompute all the fixed MDS table.  This only needs to be done
*	one time at initialization, after which the table is "CONST".
*
-****************************************************************************/
void
BuildMDS( void )
{
	int i;
	DWORD d;
	BYTE m1[2], mX[2], mY[4];
	
	for ( i = 0; i < 256; ++i )
		{
		m1[0] = P8x8[0][i];		/* compute all the matrix elements */
		mX[0] = (BYTE) Mul_X( m1[0] );
		mY[0] = (BYTE) Mul_Y( m1[0] );

		m1[1] = P8x8[1][i];
		mX[1] = (BYTE) Mul_X( m1[1] );
		mY[1] = (BYTE) Mul_Y( m1[1] );

#undef	Mul_1					/* change what the pre-processor does with Mij */
#undef	Mul_X
#undef	Mul_Y
#define	Mul_1	m1				/* It will now access m01[], m5B[], and mEF[] */
#define	Mul_X	mX				
#define	Mul_Y	mY

#define	SetMDS(N)					\
		b0(d) = M0##N[ P_##N##0 ];	\
		b1(d) = M1##N[ P_##N##0 ];	\
		b2(d) = M2##N[ P_##N##0 ];	\
		b3(d) = M3##N[ P_##N##0 ];	\
		MDStab[N][i] = d;

		SetMDS( 0 );			/* fill in the matrix with elements computed above */
		SetMDS( 1 );
		SetMDS( 2 );
		SetMDS( 3 );
		}
#undef	Mul_1
#undef	Mul_X
#undef	Mul_Y
#define	Mul_1	Mx_1			/* re-enable true multiply */
#define	Mul_X	Mx_X
#define	Mul_Y	Mx_Y
	
#if BIG_TAB
	{
	int j,k;
	BYTE * q0, * q1;

	for ( i = 0; i < 4; ++i )
		{
		switch (i)
			{
			case 0:	q0=p8(01); q1=p8(02);	break;
			case 1:	q0=p8(11); q1=p8(12);	break;
			case 2:	q0=p8(21); q1=p8(22);	break;
			case 3:	q0=p8(31); q1=p8(32);	break;
			}
		for ( j = 0; j < 256; ++j )
			for ( k = 0; k < 256; ++k )
				bigTab[i][j][k] = q0[ q1[k] ^ j ];
		}
	}
#endif

	bIsMDSInitialized = true;			/* NEVER modify the table again! */
}


/*
+*****************************************************************************
*
* Function Name:	ReverseRoundSubkeys
*
* Function:			Reverse order of round subkeys to switch between encrypt/decrypt
*
* Arguments:		key		=	ptr to keyInstance to be reversed
*					newDir	=	new direction value
*
* Return:			None.
*
* Notes:
*	This optimization allows both blockEncrypt and blockDecrypt to use the same
*	"fallthru" switch statement based on the number of rounds.
*	Note that key->numRounds must be even and >= 2 here.
*
-****************************************************************************/
void
ReverseRoundSubkeys( keyInstance * key, BYTE newDir )
{
	DWORD t0, t1;
	DWORD * r0 = key->subKeys + ROUND_SUBKEYS;
	DWORD * r1 = r0 + 2 * key->numRounds - 2;

	for ( ; r0 < r1; r0 += 2, r1 -= 2 )
		{
		t0    = r0[0];		/* swap the order */
		t1    = r0[1];
		r0[0] = r1[0];		/* but keep relative order within pairs */
		r0[1] = r1[1];
		r1[0] = t0;
		r1[1] = t1;
		}

	key->direction = newDir;
}


/*
+*****************************************************************************
*
* Function Name:	Xor256
*
* Function:			Copy an 8-bit permutation (256 bytes), xoring with a byte
*
* Arguments:		dst		=	where to put result
*					src		=	where to get data (can be same asa dst)
*					b		=	byte to xor
*
* Return:			None
*
-****************************************************************************/
void
Xor256( DWORD * dst, const DWORD * src, BYTE b )
{
#if 1
	// let's try the simple idiom first, and leave the optimizations to the compiler.
	
	// replicate byte to all four bytes
	const DWORD smear = b * 0x01010101u;
	
	for ( int i = int( 256 * sizeof(BYTE) / sizeof(DWORD) ); i > 0; --i, ++dst, ++src )
		{
		*dst = *src ^ smear;
		}
	
#else
	
	const DWORD x = b * 0x01010101u;	/* replicate byte to all four bytes */
	DWORD * d = dst;
	const DWORD * s = src;
	
#define X_8(N) { \
	  d[ (N)     ] = s[ (N)     ] ^ x; \
	  d[ (N) + 1 ] = s[ (N) + 1 ] ^ x; \
	  }
#define X_32(N)	{ \
	  X_8( (N)     ); \
      X_8( (N) + 2 ); \
	  X_8( (N) + 4 ); \
      X_8( (N) + 6 ); \
	  }
	
	X_32(  0 );
	X_32(  8 );
	X_32( 16 );
	X_32( 24 );	/* all inline */
	
	d += 32;	/* keep offsets small! */
	s += 32;
	
	X_32(  0 );
	X_32(  8 );
	X_32( 16 );
	X_32( 24 );	/* all inline */
#endif	// 1
}


/*
+*****************************************************************************
*
* Function Name:	reKey
*
* Function:			Initialize the Twofish key schedule from key32
*
* Arguments:		key			=	ptr to keyInstance to be initialized
*
* Return:			TRUE on success
*
* Notes:
*	Here we precompute all the round subkeys, although that is not actually
*	required.  For example, on a smartcard, the round subkeys can 
*	be generated on-the-fly	using f32()
*
-****************************************************************************/
int
reKey( keyInstance * key )
{
	int		i, j, k64Cnt, keyLen;
	int		subkeyCnt;
	DWORD	A = 0, B = 0, q;
	DWORD	sKey[ MAX_KEY_BITS/64 ] = {};
	DWORD	k32e[ MAX_KEY_BITS/64 ] = {};
	DWORD	k32o[ MAX_KEY_BITS/64 ] = {};
	BYTE	L0[ 256 ] ALIGN_ME = {};
	BYTE	L1[ 256 ] ALIGN_ME ;	/* small local 8-bit permutations */
	
#if VALIDATE_PARMS
  #if ALIGN32
	if ( ((int)key) & 3)
		return BAD_ALIGN32;
	if ( (key->keyLen % 64) || (key->keyLen < MIN_KEY_BITS) )
		return BAD_KEY_INSTANCE;
  #endif
#endif
	
	if ( ! bIsMDSInitialized )			/* do this one time only */
		BuildMDS();
	
#define	F32(res,x,k32)	\
	{															\
	DWORD t = x;												\
	switch ( k64Cnt & 3)										\
	    {														\
		case 0:  /* same as 4 */								\
					b0(t) = p8(04)[ b0(t) ] ^ b0( k32[3] );		\
					b1(t) = p8(14)[ b1(t) ] ^ b1( k32[3] );		\
					b2(t) = p8(24)[ b2(t) ] ^ b2( k32[3] );		\
					b3(t) = p8(34)[ b3(t) ] ^ b3( k32[3] );		\
				 /* fall thru, having pre-processed t */		\
					FALLTHRU_OK;								\
		case 3:		b0(t) = p8(03)[ b0(t) ] ^ b0( k32[2] );		\
					b1(t) = p8(13)[ b1(t) ] ^ b1( k32[2] );		\
					b2(t) = p8(23)[ b2(t) ] ^ b2( k32[2] );		\
					b3(t) = p8(33)[ b3(t) ] ^ b3( k32[2] );		\
				 /* fall thru, having pre-processed t */		\
					FALLTHRU_OK;								\
		case 2:	 /* 128-bit keys (optimize for this case) */	\
			res =	MDStab[0][ p8(01)[ p8(02)[ b0(t) ] ^ b0( k32[1] ) ] ^ b0( k32[0] ) ] ^	\
					MDStab[1][ p8(11)[ p8(12)[ b1(t) ] ^ b1( k32[1] ) ] ^ b1( k32[0] ) ] ^	\
					MDStab[2][ p8(21)[ p8(22)[ b2(t) ] ^ b2( k32[1] ) ] ^ b2( k32[0] ) ] ^	\
					MDStab[3][ p8(31)[ p8(32)[ b3(t) ] ^ b3( k32[1] ) ] ^ b3( k32[0] ) ] ;	\
		}														\
	}

#if ! CHECK_TABLE
#if defined( USE_ASM )				/* only do this if not using assember */
if ( !(useAsm & 4) )
#endif
#endif
	{
	subkeyCnt = ROUND_SUBKEYS + 2 * key->numRounds;
	keyLen = key->keyLen;
	k64Cnt = ( keyLen + 63 ) / 64;			/* number of 64-bit key words */
	for ( i = 0, j = k64Cnt - 1; i < k64Cnt; ++i, --j )
		{							/* split into even/odd key dwords */
		k32e[i] = key->key32[ 2 * i     ];
		k32o[i] = key->key32[ 2 * i + 1 ];
		
		/* compute S-box keys using (12,8) Reed-Solomon code over GF(256) */
		sKey[j] = key->sboxKeys[j] = RS_MDS_Encode( k32e[i], k32o[i] );	/* reverse order */
		}
	}

#ifdef USE_ASM
if ( useAsm & 4 )
	{
	#if defined(COMPILE_KEY) && defined(USE_ASM)
		key->keySig		= VALID_SIG;			/* show that we are initialized */
		key->codeSize	= sizeof(key->compiledCode);	/* set size */
	#endif
	reKey_86( key );
	}
else
#endif	/* USE_ASM */

	{
	for ( i = q = 0; i < subkeyCnt / 2; ++i, q += SK_STEP )	
		{									/* compute round subkeys for PHT */
		F32( A, q          , k32e );		/* A uses even key dwords */
		F32( B, q + SK_BUMP, k32o );		/* B uses odd  key dwords */
		B = ROL( B, 8 );
		key->subKeys[ 2 * i     ] = A + B;	/* combine with a PHT */
		B = A + 2 * B;
		key->subKeys[ 2 * i + 1 ] = ROL( B, SK_ROTL );
		}
	
#if !defined( ZERO_KEY )
	switch ( keyLen )	/* case out key length for speed in generating S-boxes */
		{
		case 128:
		#if defined( FULL_KEY ) || defined( PART_KEY )
#if BIG_TAB
			#define	one128(N,J)	sbSet( N, i, J, L0[i+J] )
			
			#define	sb128( N ) {						\
				BYTE * qq = bigTab[N][ b##N(sKey[1]) ];	\
				Xor256( L0, qq, b##N(sKey[0]) );		\
				for ( i = 0; i < 256; i += 2 ) { one128( N, 0 ); one128( N, 1 ); } }
			
#else
			#define	one128(N,J)	sbSet( N, uint(i), J, p8(N##1)[ L0[i + J]] ^ k0 )
			
			#define	sb128(N) {														\
				Xor256( (DWORD *) L0, (const DWORD *) p8(N##2), b##N(sKey[1]) );	\
				{ DWORD k0 = b##N( sKey[0] );								\
				for ( i = 0; i < 256; i += 2 ) { one128( N, 0 ); one128( N, 1 ); } } }
			
#endif	/* BIG_TAB */
		
		#elif defined( MIN_KEY )
		
			#define	sb128(N) Xor256( (DWORD *) _sBox8_(N), (const DWORD *) p8(N##2), b##N(sKey[1]) )
		
		#endif	// FULL_KEY || PART_KEY || MIN_KEY
			
			sb128( 0 );
			sb128( 1 );
			sb128( 2 );
			sb128( 3 );
			break;
		
		case 192:
		
		#if defined(FULL_KEY) || defined(PART_KEY)
			#define one192(N,J) sbSet( N, uint(i), J, p8(N##1)[ p8(N##2)[ L0[ i + J ] ] ^ k1 ] ^ k0 )
			
			#define	sb192(N) {													\
				Xor256( (DWORD *) L0, (const DWORD *) p8(N##3),b##N(sKey[2]));	\
				{ DWORD k0 = b##N( sKey[0] );							\
				  DWORD k1 = b##N( sKey[1] );							\
				  for (i = 0; i < 256; i += 2) { one192( N, 0 ); one192( N, 1 ); } } }
			
		#elif defined(MIN_KEY)
			
			#define one192(N,J) sbSet( N, i, J, p8(N##2)[ L0[ i + J ]] ^ k1 )
			
			#define	sb192(N) {								\
				Xor256( L0, p8(N##3), b##N( sKey[2] ) );	\
				{ DWORD k1 = b##N( sKey[1] );		\
				  for ( i = 0; i < 256; i += 2 ) { one192( N, 0 ); one192( N, 1 ); } } }
		
		#endif	// FULL_KEY || PART_KEY || MIN_KEY
		
			sb192( 0 );
			sb192( 1 );
			sb192( 2 );
			sb192( 3 );
			break;
		
		case 256:
		
		#if defined(FULL_KEY) || defined(PART_KEY)
			
			#define one256(N,J) sbSet( N, uint(i), J, p8(N##1)[ p8(N##2)[ L0[ i + J ]] ^ k1 ] ^ k0 )
			
			#define	sb256(N) {															\
				Xor256( (DWORD *) L1, (const DWORD *) p8( N##4 ), b##N( sKey[3] ) );	\
				for ( i = 0; i <256 ; i += 2 ) { 										\
					L0[ i     ] = p8(N##3)[ L1[ i     ] ];								\
					L0[ i + 1 ] = p8(N##3)[ L1[ i + 1 ] ]; }							\
				Xor256( (DWORD *) L0, (const DWORD *) L0, b##N(sKey[2]) );				\
				{ DWORD k0 = b##N( sKey[0] );									\
				  DWORD k1 = b##N( sKey[1] );									\
				  for ( i = 0; i < 256; i += 2 ) { one256( N, 0 ); one256( N, 1 ); } } }
			
		#elif defined(MIN_KEY)
			
			#define one256(N,J) sbSet( N, i, J, p8(N##2)[ L0[ i + J ] ] ^ k1 )
			
			#define	sb256(N) {									\
				Xor256( L1, p8(N##4), b##N( sKey[3] ) );		\
				for ( i = 0; i < 256; i += 2 ) {				\
					L0[i  ] = p8(N##3)[ L1[ i ] ];				\
					L0[ i + 1] = p8(N##3)[ L1[ i + 1 ] ]; }		\
				Xor256( L0, L0, b##N( sKey[2] ) );				\
				{ DWORD k1 = b##N( sKey[1] );			\
				  for ( i = 0; i < 256; i += 2 ) { one256( N, 0 ); one256( N, 1 ); } } }
			
		#endif	/* FULL_KEY || PART_KEY || MIN_KEY */
			
			sb256( 0 );
			sb256( 1 );
			sb256( 2 );
			sb256( 3 );
			break;
		}
#endif	/* ! defined(ZERO_KEY) */
	}
	
#if CHECK_TABLE						/* sanity check  vs. pedagogical code*/
	{
	GetSboxKey;
	for ( i = 0; i < subkeyCnt / 2; ++i )
		{
		A = f32( i * SK_STEP          , k32e, keyLen );	/* A uses even key dwords */
		B = f32( i * SK_STEP + SK_BUMP, k32o, keyLen );	/* B uses odd  key dwords */
		B = ROL( B, 8 );
		assert( key->subKeys[ 2 * i    ] ==      A + B );
		assert( key->subKeys[ 2 * i + 1] == ROL( A + 2 * B, SK_ROTL ) );
		}
  #if !defined(ZERO_KEY)			/* any S-boxes to check? */
	for ( i = q = 0; i < 256; ++i, q += 0x01010101 )
		assert( f32( q, key->sboxKeys, keyLen ) == Fe32_( q, 0) );
  #endif
	}
#endif /* CHECK_TABLE */
	
	DebugDumpKey( key );
	
	if ( key->direction == DIR_ENCRYPT )	
		ReverseRoundSubkeys( key, DIR_ENCRYPT );	/* reverse the round subkey order */
	
	return TRUE;
}


/*
+*****************************************************************************
*
* Function Name:	makeKey
*
* Function:			Initialize the Twofish key schedule
*
* Arguments:		key			=	ptr to keyInstance to be initialized
*					direction	=	DIR_ENCRYPT or DIR_DECRYPT
*					keyLen		=	# bits of key text at *keyMaterial
*					keyMaterial	=	ptr to hex ASCII chars representing key bits
*
* Return:			TRUE on success
*					else error code (e.g., BAD_KEY_DIR)
*
* Notes:	This parses the key bits from keyMaterial.  Zeroes out unused key bits
*
-****************************************************************************/
int
makeKey( keyInstance * key, BYTE direction, int keyLen, const char * keyMaterial )
{
#if VALIDATE_PARMS				/* first, sanity check on parameters */
	if ( key == NULL )			
		return BAD_KEY_INSTANCE;/* must have a keyInstance to initialize */
	if ( (direction != DIR_ENCRYPT) && (direction != DIR_DECRYPT) )
		return BAD_KEY_DIR;		/* must have valid direction */
	if ( (keyLen > MAX_KEY_BITS) || (keyLen < 8) || (keyLen & 0x3F) )
		return BAD_KEY_MAT;		/* length must be valid */
	key->keySig = VALID_SIG;	/* show that we are initialized */
  #if ALIGN32
	if ( ( ( (int)key ) & 3) || ( ( (int)key->key32 ) & 3) )
		return BAD_ALIGN32;
  #endif
#endif

	key->direction	= direction;					/* set our cipher direction */
	key->keyLen		= ( keyLen + 63) & ~63;			/* round up to multiple of 64 */
	key->numRounds	= numRounds[ (keyLen - 1)/64 ];
	memset( key->key32, 0 , sizeof key->key32 );	/* zero unused bits */
	key->keyMaterial[ MAX_KEY_SIZE ] = 0;			/* terminate ASCII string */

	if ( ( keyMaterial == NULL ) || ( keyMaterial[0]==0 ) )
		return TRUE;								/* allow a "dummy" call */
	
	// DTS: we always use the recommended reKey() approach, so we can bail now.
	return FALSE;
#if 0
	if ( ParseHexDword( keyLen, keyMaterial, key->key32, key->keyMaterial ) )
		return BAD_KEY_MAT;	
	
	return reKey( key );							/* generate round subkeys */
#endif
}


/*
+*****************************************************************************
*
* Function Name:	cipherInit
*
* Function:			Initialize the Twofish cipher in a given mode
*
* Arguments:		cipher		=	ptr to cipherInstance to be initialized
*					mode		=	MODE_ECB, MODE_CBC, or MODE_CFB1
*					IV			=	ptr to hex ASCII test representing IV bytes
*
* Return:			TRUE on success
*					else error code (e.g., BAD_CIPHER_MODE)
*
-****************************************************************************/
int
cipherInit( cipherInstance * cipher, BYTE mode, const char * IV )
{
#if VALIDATE_PARMS				/* first, sanity check on parameters */
	if ( cipher == NULL )			
		return BAD_PARAMS;		/* must have a cipherInstance to initialize */
	if ( (mode != MODE_ECB) && (mode != MODE_CBC) && (mode != MODE_CFB1) )
		return BAD_CIPHER_MODE;	/* must have valid cipher mode */
	cipher->cipherSig	=	VALID_SIG;
  #if ALIGN32
	if ( (((int)cipher) & 3) || (((int)cipher->IV) & 3) || (((int)cipher->iv32) & 3) )
		return BAD_ALIGN32;
  #endif
#endif
	
	if ( (mode != MODE_ECB) && (IV) )	/* parse the IV */
		{
		// DTS: again, we never exercise this code path.
		return FALSE;
#if 0
		if ( ParseHexDword( BLOCK_SIZE, IV, cipher->iv32, NULL ) )
			return BAD_IV_MAT;
		for ( int i = 0; i < BLOCK_SIZE / 32; ++i )	/* make byte-oriented copy for CFB1 */
			{
			DWORD temp = Bswap( cipher->iv32[i] );
			( (DWORD *)cipher->IV )[ i ] = temp;
			}
#endif
		}
	
	cipher->mode		=	mode;
	
	return TRUE;
}

/*
+*****************************************************************************
*
* Function Name:	blockEncrypt
*
* Function:			Encrypt block(s) of data using Twofish
*
* Arguments:		cipher		=	ptr to already initialized cipherInstance
*					key			=	ptr to already initialized keyInstance
*					input		=	ptr to data blocks to be encrypted
*					inputLen	=	# bits to encrypt (multiple of blockSize)
*					outBuffer	=	ptr to where to put encrypted blocks
*
* Return:			# bits ciphered (>= 0)
*					else error code (e.g., BAD_CIPHER_STATE, BAD_KEY_MATERIAL)
*
* Notes: The only supported block size for ECB/CBC modes is BLOCK_SIZE bits.
*		 If inputLen is not a multiple of BLOCK_SIZE bits in those modes,
*		 an error BAD_INPUT_LEN is returned.  In CFB1 mode, all block 
*		 sizes can be supported.
*
-****************************************************************************/
int
blockEncrypt( cipherInstance * cipher, keyInstance * key, const BYTE * input,
				int inputLen, BYTE * outBuffer )
{
	int   i, n;							/* loop counters */
	DWORD x[ BLOCK_SIZE / 32 ] = {};	/* block being encrypted */
	DWORD t0, t1;						/* temp variables */
	int	  rounds = key->numRounds;		/* number of rounds */
	BYTE  bit, bit0, ctBit, carry;		/* temps for CFB */

	/* make local copies of things for faster access */
	int	  mode = cipher->mode;
	DWORD sk[ TOTAL_SUBKEYS ];
	DWORD IV[ BLOCK_SIZE / 32 ];
	
	GetSboxKey;

#if VALIDATE_PARMS
	if ((cipher == NULL) || (cipher->cipherSig != VALID_SIG))
		return BAD_CIPHER_STATE;
	if ((key == NULL) || (key->keySig != VALID_SIG))
		return BAD_KEY_INSTANCE;
	if ((rounds < 2) || (rounds > MAX_ROUNDS) || (rounds&1))
		return BAD_KEY_INSTANCE;
	if ((mode != MODE_CFB1) && (inputLen % BLOCK_SIZE))
		return BAD_INPUT_LEN;
  #if ALIGN32
	if ( (((int)cipher) & 3) || (((int)key      ) & 3) ||
		 (((int)input ) & 3) || (((int)outBuffer) & 3))
		return BAD_ALIGN32;
  #endif
#endif

	if ( mode == MODE_CFB1 )
		{	/* use recursion here to handle CFB, one block at a time */
		cipher->mode = MODE_ECB;	/* do encryption in ECB */
		for ( n = 0; n < inputLen; ++n )
			{
			blockEncrypt( cipher, key, cipher->IV, BLOCK_SIZE, (BYTE *) x );
			bit0  = BYTE(0x80 >> (n & 7));		/* which bit position in byte */
			ctBit = BYTE(( input[n/8] & bit0 ) ^ ((((BYTE *) x)[0] & 0x80) >> (n&7)));
			outBuffer[n/8] = BYTE((outBuffer[n/8] & ~ bit0) | ctBit);
			carry = BYTE(ctBit >> (7 - (n&7)));
			for ( i = BLOCK_SIZE/8 - 1; i >= 0 ; --i )
				{
				bit = BYTE(cipher->IV[i] >> 7);	/* save next "carry" from shift */
				cipher->IV[i] = BYTE((cipher->IV[i] << 1) ^ carry);
				carry = bit;
				}
			}
		cipher->mode = MODE_CFB1;	/* restore mode for next time */
		return inputLen;
		}

	/* here for ECB, CBC modes */
	if ( key->direction != DIR_ENCRYPT )
		ReverseRoundSubkeys( key, DIR_ENCRYPT );	/* reverse the round subkey order */
	
#ifdef USE_ASM
	if ( (useAsm & 1) && (inputLen) )
  #ifdef COMPILE_KEY
		if ( key->keySig == VALID_SIG )
			return ( (CipherProc *)(key->encryptFuncPtr))( cipher, key, input, inputLen, outBuffer );
  #else
		return (*blockEncrypt_86)(cipher, key, input, inputLen, outBuffer );
  #endif
#endif
	/* make local copy of subkeys for speed */
	memcpy( sk, key->subKeys, sizeof(DWORD) * uint( ROUND_SUBKEYS + 2 * rounds ) );
	if ( mode == MODE_CBC )
		BlockCopy( IV, cipher->iv32 )
	else
		IV[0] = IV[1] = IV[2] = IV[3] = 0;
	
	for ( n = 0; n < inputLen; n += BLOCK_SIZE, input += BLOCK_SIZE / 8, outBuffer += BLOCK_SIZE / 8 )
		{
#ifdef DEBUG_ENCRYPTION
		DebugDump( input, "\n", -1, 0, 0, 0, 1 );
		if ( cipher->mode == MODE_CBC )
			DebugDump( cipher->iv32, "" , IV_ROUND, 0, 0, 0, 0 );
#endif
#define	LoadBlockE(N)  x[N]=Bswap( ( (const DWORD *)input)[N] ) ^ sk[INPUT_WHITEN + N] ^ IV[N]
		LoadBlockE( 0 );
		LoadBlockE( 1 );
		LoadBlockE( 2 );
		LoadBlockE( 3 );
		DebugDump( x, "", 0, 0, 0, 0, 0 );
#define	EncryptRound(K,R,id)	\
			t0	    = Fe32##id( x[ K     ], 0 );						\
			t1	    = Fe32##id( x[ K ^ 1 ], 3 );						\
			x[K^3]  = ROL( x[ K ^ 3 ], 1 );								\
			x[K^2] ^= t0 +     t1 + sk[ ROUND_SUBKEYS + 2 * (R)     ];	\
			x[K^3] ^= t0 + 2 * t1 + sk[ ROUND_SUBKEYS + 2 * (R) + 1 ];	\
			x[K^2]  = ROR( x[ K ^ 2 ], 1 );								\
			DebugDump( x, "", rounds - (R), 0, 0, 1, 0 );
#define		Encrypt2(R,id)	{ EncryptRound( 0, R + 1, id ); EncryptRound( 2, R, id ); }

#if defined( ZERO_KEY )
		switch ( key->keyLen )
			{
			case 128:
				for ( i = rounds - 2; i >= 0; i -= 2 )
					Encrypt2( i, _128 );
				break;
			case 192:
				for ( i = rounds - 2; i >= 0; i -= 2 )
					Encrypt2( i, _192 );
				break;
			case 256:
				for ( i = rounds - 2; i >= 0; i -= 2 )
					Encrypt2( i, _256 );
				break;
			}
#else
		Encrypt2( 14, _ );
		Encrypt2( 12, _ );
		Encrypt2( 10, _ );
		Encrypt2(  8, _ );
		Encrypt2(  6, _ );
		Encrypt2(  4, _ );
		Encrypt2(  2, _ );
		Encrypt2(  0, _ );
#endif

		/* need to do (or undo, depending on your point of view) final swap */
#if LittleEndian
#define	StoreBlockE(N)	((DWORD *)outBuffer)[N]=x[N^2] ^ sk[OUTPUT_WHITEN + N]
#else
#define	StoreBlockE(N)	{ t0=x[ N ^ 2 ] ^ sk[ OUTPUT_WHITEN + N ]; ((DWORD *)outBuffer)[N] = Bswap( t0 ); }
#endif
		StoreBlockE( 0 );
		StoreBlockE( 1 );
		StoreBlockE( 2 );
		StoreBlockE( 3 );
		
		if ( mode == MODE_CBC )
			{
			IV[0] = Bswap( ((DWORD *)outBuffer )[ 0 ] );
			IV[1] = Bswap( ((DWORD *)outBuffer )[ 1 ] );
			IV[2] = Bswap( ((DWORD *)outBuffer )[ 2 ] );
			IV[3] = Bswap( ((DWORD *)outBuffer )[ 3 ] );
			}
#ifdef DEBUG_ENCRYPTION
		DebugDump( outBuffer, "", rounds + 1, 0, 0, 0, 1 );
		if ( cipher->mode == MODE_CBC )
			DebugDump( cipher->iv32, "", IV_ROUND, 0, 0, 0, 0 );
#endif
		}

	if ( mode == MODE_CBC )
		BlockCopy( cipher->iv32, IV );
	
	return inputLen;
}


/*
+*****************************************************************************
*
* Function Name:	blockDecrypt
*
* Function:			Decrypt block(s) of data using Twofish
*
* Arguments:		cipher		=	ptr to already initialized cipherInstance
*					key			=	ptr to already initialized keyInstance
*					input		=	ptr to data blocks to be decrypted
*					inputLen	=	# bits to encrypt (multiple of blockSize)
*					outBuffer	=	ptr to where to put decrypted blocks
*
* Return:			# bits ciphered (>= 0)
*					else error code (e.g., BAD_CIPHER_STATE, BAD_KEY_MATERIAL)
*
* Notes: The only supported block size for ECB/CBC modes is BLOCK_SIZE bits.
*		 If inputLen is not a multiple of BLOCK_SIZE bits in those modes,
*		 an error BAD_INPUT_LEN is returned.  In CFB1 mode, all block 
*		 sizes can be supported.
*
-****************************************************************************/
int
blockDecrypt( cipherInstance * cipher, keyInstance * key, const BYTE * input,
				int inputLen, BYTE * outBuffer )
{
	int   i, n;							/* loop counters */
	DWORD x[ BLOCK_SIZE / 32 ] = {};	/* block being encrypted */
	DWORD t0, t1;						/* temp variables */
	int	  rounds = key->numRounds;		/* number of rounds */
	BYTE  bit, bit0, ctBit, carry;		/* temps for CFB */
	
	/* make local copies of things for faster access */
	int	  mode = cipher->mode;
	DWORD sk[ TOTAL_SUBKEYS ];
	DWORD IV[ BLOCK_SIZE / 32 ];
	
	GetSboxKey;
	
#if VALIDATE_PARMS
	if ( (cipher == NULL) || (cipher->cipherSig != VALID_SIG) )
		return BAD_CIPHER_STATE;
	if ( (key == NULL) || (key->keySig != VALID_SIG) )
		return BAD_KEY_INSTANCE;
	if ( (rounds < 2) || (rounds > MAX_ROUNDS) || (rounds&1) )
		return BAD_KEY_INSTANCE;
	if ( (cipher->mode != MODE_CFB1) && (inputLen % BLOCK_SIZE) )
		return BAD_INPUT_LEN;
  #if ALIGN32
	if ( (((int)cipher) & 3) || (((int)key      ) & 3) ||
		 (((int)input)  & 3) || (((int)outBuffer) & 3))
		return BAD_ALIGN32;
  #endif
#endif

	if ( cipher->mode == MODE_CFB1 )
		{	/* use blockEncrypt here to handle CFB, one block at a time */
		cipher->mode = MODE_ECB;	/* do encryption in ECB */
		for ( n = 0; n < inputLen; ++n )
			{
			blockEncrypt( cipher, key, cipher->IV, BLOCK_SIZE, (BYTE *) x );
			bit0  = BYTE(0x80 >> (n & 7));
			ctBit = input[n/8] & bit0;
			outBuffer[n/8] = BYTE(( outBuffer[n/8] & ~ bit0 ) |
								  ( ctBit ^ ((((BYTE *) x)[0] & 0x80) >> (n&7) )));
			carry = BYTE(ctBit >> ( 7 - (n & 7) ));
			for ( i = BLOCK_SIZE / 8 - 1; i >= 0; --i )
				{
				bit = BYTE(cipher->IV[i] >> 7);	/* save next "carry" from shift */
				cipher->IV[i] = BYTE(( cipher->IV[i] << 1 ) ^ carry);
				carry = bit;
				}
			}
		cipher->mode = MODE_CFB1;	/* restore mode for next time */
		return inputLen;
		}
	
	/* here for ECB, CBC modes */
	if ( key->direction != DIR_DECRYPT )
		ReverseRoundSubkeys( key, DIR_DECRYPT );	/* reverse the round subkey order */
	
#ifdef USE_ASM
	if ( (useAsm & 2) && (inputLen) )
  #ifdef COMPILE_KEY
		if ( key->keySig == VALID_SIG )
			return ((CipherProc *)(key->decryptFuncPtr))(cipher,key,input,inputLen,outBuffer);
  #else	
		return (*blockDecrypt_86)( cipher, key, input, inputLen, outBuffer );
  #endif
#endif	/* USE_ASM */
	
	/* make local copy of subkeys for speed */
	memcpy( sk, key->subKeys, sizeof(DWORD) * uint( ROUND_SUBKEYS + 2 * rounds ) );
	
	if ( mode == MODE_CBC )
		BlockCopy( IV, cipher->iv32 )
	else
		IV[0] = IV[1] = IV[2] = IV[3] = 0;
	
	for ( n = 0; n < inputLen; n += BLOCK_SIZE, input += BLOCK_SIZE / 8, outBuffer += BLOCK_SIZE / 8 )
		{
		DebugDump( input, "\n", rounds + 1 , 0, 0, 0, 1 );
		
#define LoadBlockD(N) x[ N ^ 2 ] = Bswap( ((const DWORD *)input)[ N ]) ^ sk[ OUTPUT_WHITEN + N ]
		
		LoadBlockD( 0 );
		LoadBlockD( 1 );
		LoadBlockD( 2 );
		LoadBlockD( 3 );
		
#define	DecryptRound(K,R,id)									\
			t0	   = Fe32##id( x[ K     ], 0 );					\
			t1	   = Fe32##id( x[ K ^ 1 ], 3 );					\
			DebugDump( x, "", (R) + 1, 0, 0, 1, 0 );			\
			x[ K ^ 2 ]  = ROL( x[ K ^ 2 ], 1 );					\
			x[ K ^ 2 ] ^= t0 +     t1 + sk[ ROUND_SUBKEYS + 2 * (R)     ];	\
			x[ K ^ 3 ] ^= t0 + 2 * t1 + sk[ ROUND_SUBKEYS + 2 * (R) + 1 ];	\
			x[ K ^ 3 ]  = ROR( x[ K ^ 3], 1 );					\
		
#define		Decrypt2(R,id)	{ DecryptRound( 2, R + 1, id ); DecryptRound( 0, R, id ); }
		
#if defined(ZERO_KEY)
		switch ( key->keyLen )
			{
			case 128:
				for ( i = rounds - 2; i >= 0; i -= 2 )
					Decrypt2( i, _128 );
				break;
			case 192:
				for ( i = rounds - 2; i >= 0; i -= 2 )
					Decrypt2( i, _192 );
				break;
			case 256:
				for ( i = rounds - 2; i >= 0; i -= 2 )
					Decrypt2( i, _256 );
				break;
			}
#else
		{
		Decrypt2( 14, _ );
		Decrypt2( 12, _ );
		Decrypt2( 10, _ );
		Decrypt2(  8, _ );
		Decrypt2(  6, _ );
		Decrypt2(  4, _ );
		Decrypt2(  2, _ );
		Decrypt2(  0, _ );
		}
#endif
		DebugDump( x, "", 0, 0, 0, 0, 0 );
		if ( cipher->mode == MODE_ECB )
			{
#if LittleEndian
#define	StoreBlockD(N)	((DWORD *)outBuffer)[N] = x[N] ^ sk[INPUT_WHITEN+N]
#else
#define	StoreBlockD(N)	{ t0 = x[N] ^ sk[ INPUT_WHITEN + N ]; ( (DWORD *)outBuffer )[N] = Bswap( t0 ); }
#endif
			StoreBlockD( 0 );
			StoreBlockD( 1 );
			StoreBlockD( 2 );
			StoreBlockD( 3 );
#undef  StoreBlockD
			
			DebugDump( outBuffer, "", -1, 0, 0, 0, 1 );
			continue;
			}
		else
			{
#define	StoreBlockD(N)	x[N]   ^= sk[ INPUT_WHITEN + N ] ^ IV[N];	\
						IV[N]   = Bswap( ((const DWORD *)input)[N]);	\
						( (DWORD *)outBuffer)[N] = Bswap( x[N] );
			StoreBlockD( 0 );
			StoreBlockD( 1 );
			StoreBlockD( 2 );
			StoreBlockD( 3 );
#undef  StoreBlockD
			
			DebugDump( outBuffer, "", -1, 0, 0, 0, 1 );
			}
		}
	
	if ( mode == MODE_CBC )	/* restore iv32 to cipher */
		BlockCopy( cipher->iv32, IV )
	
	return inputLen;
}


#if 0
DWORD
TwofishCodeSize( void )
{
	DWORD x = Here( 0 );
#ifdef USE_ASM
	if ( useAsm & 3 )
		return TwofishAsmCodeSize();
#endif
	return x - TwofishCodeStart();
}
#endif	// 0


#pragma mark -

//
// NOTE!
// The input {plain,cipher}text for these functions MUST be a multiple
// of 16 bytes long, else you'll encounter innumerable mysterious glitches.
// The keytext can safely be of any (non-zero) length -- but, needless to say,
// short keys don't provide much security.
//

/*
**	DTSEncode()
**
**	encrypt data using the key
*/
DTSError
DTSEncode(	const void *	plaintext,
			void * 			ciphertext,
			size_t			textlen,
			const void *	key,
			size_t			keysize )
{
	// convert the arbitrary key to a random-looking jumble of bits
	uchar hash[16];
	DTSOneWayHash( key, keysize, hash );
	
	// set up the key
	keyInstance ki;				// holds key information, including tables
	const int keylenbits = int( 8 * sizeof hash );
	if ( makeKey( &ki, DIR_ENCRYPT, keylenbits, nullptr ) != TRUE )
		return -1;
	
	// set up the cipher
	cipherInstance ci;			// holds mode (ECB, CBC) and IV
	if ( cipherInit( &ci, MODE_ECB, nullptr ) != TRUE )
		return -1;
	
	// run the key schedule
#if DTS_BIG_ENDIAN
	memcpy( ki.key32, hash, sizeof hash );
#else
	{
	const uint32_t * src = reinterpret_cast<uint32_t *>( hash );
	uint32_t * dst = ki.key32;
	*dst++ = BigToNativeEndian( *src++ );
	*dst++ = BigToNativeEndian( *src++ );
	*dst++ = BigToNativeEndian( *src++ );
	*dst++ = BigToNativeEndian( *src++ );
	}
#endif  // ! DTS_BIG_ENDIAN
	
	reKey( &ki );
	
	// encrypt the bytes
	int textlenbits = int( textlen * 8 );
	if ( textlenbits != blockEncrypt( &ci, &ki,
							static_cast<const uchar *>( plaintext ),
							textlenbits,
							static_cast<uchar *>( ciphertext ) ) )
		return -1;
	
	return noErr;
}


/*
**	DTSDecode()
**
**	decrypt data using the key
*/
DTSError
DTSDecode(	const void *	ciphertext,
			void *			plaintext,
			size_t			textlen,
			const void *	key,
			size_t			keysize )
{
	// convert the arbitrary key to a random-looking jumble of bits
	uchar hash[16];
	DTSOneWayHash( key, keysize, hash );
	
	// set up the key
	keyInstance ki;			// key information, including tables
	const int keylenbits = int( 8 * sizeof hash );
	if ( makeKey( &ki, DIR_ENCRYPT, keylenbits, nullptr ) != TRUE )
		return -1;
	
	// set up the cipher
	cipherInstance ci;			// keeps mode (ECB, CBC) and IV
	if ( cipherInit( &ci, MODE_ECB, nullptr ) != TRUE )
		return -1;
	
	// run the key schedule
#if DTS_BIG_ENDIAN
	memcpy( ki.key32, hash, sizeof hash );
#else
	{
	const uint32_t * src = reinterpret_cast<uint32_t *>( hash );
	uint32_t * dst = ki.key32;
	*dst++ = BigToNativeEndian( *src++ );
	*dst++ = BigToNativeEndian( *src++ );
	*dst++ = BigToNativeEndian( *src++ );
	*dst++ = BigToNativeEndian( *src++ );
	}
#endif  // ! DTS_BIG_ENDIAN
	
	reKey( &ki );
	
	// decrypt the bytes
	int textlenbits = int( textlen * 8 );
	if ( textlenbits != blockDecrypt( &ci, &ki,
							static_cast<const uchar *>( ciphertext ),
							textlenbits,
							static_cast<uchar *>( plaintext ) ) )
		return -1;
	
	return noErr;
}

