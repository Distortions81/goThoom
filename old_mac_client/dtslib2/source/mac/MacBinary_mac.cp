/*
**	MacBinary_mac.cp		dtslib2
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

//################################################################################
//
//  This program is free software; you can redistribute it and/or
//  modify it under the terms of the GNU General Public License
//  as published by the Free Software Foundation; either version 2
//  of the License, or (at your option) any later version.
// 
//  This program is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
// 
//  You should have received a copy of the GNU General Public License
//  along with this program; if not, write to the Free Software
//  Foundation, Inc., 59 Temple Place - Suite 330, Boston, MA
//  02111-1307, USA.
//
//################################################################################


/* -------------------------------------------------------------------------------

	File:		MacBinary.cp
	Authors:	Josef W. Wankerl
	
	Description:
		This file contains code to encode/decode MacBinary I/II/III files.
		 
	Modification History:
		Apr. 10, 1999
		Integrated into main MacCVSPro source.

		4/28/2001
			adapted for Clan Lord, with bits filched from the MacCVSPro sources & elsewhere
		7/8/2005
			more of same
		6/2012
			converted to FSRefs

TODO: verify endian-correctness
-------------------------------------------------------------------------------- */

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif


#include "Utilities_dts.h"
#include "MacBinary_mac.h"


// utilities for fishing around within a MacBinary header
#define MBFindByte( theHeader, theOffset )	 ( * ( (const UInt8  *) ( theHeader + theOffset )) )
#define MBFindWord( theHeader, theOffset )	 ( * ( (const UInt16 *) ( theHeader + theOffset )) )
#define MBFindLong( theHeader, theOffset )	 ( * ( (const UInt32 *) ( theHeader + theOffset )) )
#define MBFindOSType( theHeader, theOffset ) ( * ( (const OSType *) ( theHeader + theOffset )) )

// field offsets within the header
enum
	{
	kOldVersionOffset			= 0,
	kFilenameLengthOffset		= 1,
	kFiletypeOffset				= 65,
	kCreatorOffset				= 69,
	kFinderFlagsHighOffset		= 73,
	kZeroFill1Offset			= 74,
	kVerticalPosOffset			= 75,
	kHorizontalPosOffset		= 77,
	kFolderIDOffset				= 79,
	kProtectedOffset			= 81,
	kZeroFill2Offset			= 82,
	kDataForkOffset				= 83,
	kResourceForkOffset			= 87,
	kCreatedDateOffset			= 91,
	kModifiedDateOffset			= 95,
	kCommentOffset				= 99,
	kFinderFlagsLowOffset		= 101,
	kMacBinarySignatureOffset	= 102,
	kScriptOffset				= 106,
	kExtendedFlagsOffset		= 107,
	kUnpackedOffset				= 116,
	kSecondaryHeaderOffset		= 120,
	kVersion2Offset				= 122,
	kMinVersionOffset			= 123,
	kChecksumOffset				= 124,
	kComputerOSOffset			= 126,
	
	kMacBinaryHeaderSize		= 128,
	kChecksumEnd				= kChecksumOffset,
	
	kVersion1ZeroFill			= 99,
	kVersion1ZeroFillLength		= 27,
	
	kVersion2					= 129,
	kVersion3					= 130
	};


const OSType kMacBinarySignature	= 'mBIN';
const uint kDefaultBufferSize 		= 128 * 1024;


// accessors to MacBinary header fields
#define MBOldVersion( theHeader )		MBFindByte( theHeader, kOldVersionOffset )
#define MBFilenameLength( theHeader )	MBFindByte( theHeader, kFilenameLengthOffset )
#define MBFiletype( theHeader )			MBFindOSType( theHeader, kFiletypeOffset )
#define MBCreator( theHeader )			MBFindOSType( theHeader, kCreatorOffset )
#define MBFinderFlagsHigh( theHeader )	MBFindByte( theHeader, kFinderFlagsHighOffset )
#define MBZeroFill1( theHeader )		MBFindByte( theHeader, kZeroFill1Offset )
#define MBVerticalPos( theHeader )		MBFindWord( theHeader, kVerticalPosOffset )
#define MBHorizontalPos( theHeader )	MBFindWord( theHeader, kHorizontalPosOffset )
#define MBFolderID( theHeader )			MBFindWord( theHeader, kFolderIDOffset )
#define MBProtected( theHeader )		MBFindByte( theHeader, kProtectedOffset )
#define MBZeroFill2( theHeader )		MBFindByte( theHeader, kZeroFill2Offset )
#define MBDataFork( theHeader )			MBFindLong( theHeader, kDataForkOffset )
#define MBResourceFork( theHeader )		MBFindLong( theHeader, kResourceForkOffset )
#define MBCreatedDate( theHeader )		MBFindLong( theHeader, kCreatedDateOffset )
#define MBModifiedDate( theHeader )		MBFindLong( theHeader, kModifiedDateOffset )
#define MBComment( theHeader )			MBFindWord( theHeader, kCommentOffset )
#define MBFinderFlagsLow( theHeader )	MBFindByte( theHeader, kFinderFlagsLowOffset )
#define MBSignature( theHeader )		MBFindOSType( theHeader, kMacBinarySignatureOffset )
#define MBScript( theHeader )			MBFindByte( theHeader, kScriptOffset )
#define MBExtendedFlags( theHeader )	MBFindByte( theHeader, kExtendedFlagsOffset )
#define MBUnpacked( theHeader )			MBFindLong( theHeader, kUnpackedOffset )
#define MBSecondaryHeader( theHeader )	MBFindWord( theHeader, kSecondaryHeaderOffset )
#define MBVersion2( theHeader )			MBFindByte( theHeader, kVersion2Offset )
#define MBMinVersion( theHeader )		MBFindByte( theHeader, kMinVersionOffset )
#define MBChecksum( theHeader )			MBFindWord( theHeader, kChecksumOffset )
#define MBComputerOS( theHeader )		MBFindWord( theHeader, kComputerOSOffset )

#define MBFinderFlags( theHeader )		( MBFinderFlagsHigh( theHeader ) << 8) + \
										  MBFinderFlagsLow( theHeader )


/*
**	class StFileAccess
**	encapsulates access to an opened file
*/
class StFileAccess
{
public:
						StFileAccess( const FSRef& inRef );
		
						~StFileAccess();
		
		OSStatus		GetFSDataForkRef( SInt8 inPerm, SInt16& outRefNum );
		
	 	OSStatus		GetFSResForkRef( SInt8 inPerm, SInt16& outRefNum );
	 	
//		OSStatus		GetResFileRef( SInt8 inPerm, SInt16& outRefNum );
		
		void			SetDelete( bool inDelete )
							{
							mDeleteOnDestroy = inDelete;
							}
		
		void			CloseAllFileRefs();
		
private:
		FSRef				mFileRef;
		SInt16				mFSDataForkRef;
		SInt16				mFSResForkRef;
//		SInt16				mResFileRef;
		bool				mDeleteOnDestroy;
};


StFileAccess::StFileAccess( const FSRef& inRef )
	:	mFileRef( inRef ),
		mFSDataForkRef(),
		mFSResForkRef(),
//		mResFileRef( kResFileNotOpened ),
		mDeleteOnDestroy( false )
{
}


StFileAccess::~StFileAccess()
{
	CloseAllFileRefs();
	
	if ( mDeleteOnDestroy )
		__Verify_noErr( FSDeleteObject( &mFileRef ) );
}


/*
**	StFileAccess::GetFSDataForkRef()
**	get refNum to opened data fork (also opens it if need be)
*/
OSStatus
StFileAccess::GetFSDataForkRef( SInt8 inPerm, SInt16& outRefNum )
{	
	if ( 0 == mFSDataForkRef )
		{
//		HFSUniStr255 dataForkName;
//		OSStatus error = FSGetDataForkName( &dataForkName );
//		if ( noErr == error )
		OSStatus error = FSOpenFork( &mFileRef,
						0, nullptr,	// aka data-fork name
						inPerm,
						&mFSDataForkRef );
		
		if ( error != noErr )
			{
			mFSDataForkRef = 0;
			return error;
			}
		}
	
	outRefNum = mFSDataForkRef;
	return noErr;
}


/*
**	StFileAccess::GetFSResForkRef()
**	get refNum to opened resource fork (also opens it if need be)
**	(this is the -raw- resource fork: ResMgr APIs will NOT work on it!)
*/
OSStatus
StFileAccess::GetFSResForkRef( SInt8 inPerm, SInt16& outRefNum )
{
	if ( 0 == mFSResForkRef )
		{
		HFSUniStr255 resForkName;
		OSStatus error = FSGetResourceForkName( &resForkName );
		if ( noErr == error )
			error = FSOpenFork( &mFileRef,
						resForkName.length,
						resForkName.unicode,
						inPerm,
						&mFSResForkRef );
		
		if ( error != noErr )
			{
			mFSResForkRef = 0;
			return error;
			}
		}
	
	outRefNum = mFSResForkRef;
	return noErr;
}



#if 0	// not needed
/*
**	StFileAccess::GetResFileRef()
**	get refNum to opened resource file (Resource Manager)
*/
OSStatus
StFileAccess::GetResFileRef( SInt8 inPerm, SInt16& outRefNum )
{
	OSStatus error = noErr;
	Handle oldTopMapHand = nullptr;
	
	if ( kResFileNotOpened == mResFileRef )
		{
		// JWW - Find out if the file is open already under Carbon
		Boolean chain;
		ResFileRefNum refNum;
		Boolean alreadyOpen = FSResourceFileAlreadyOpen( &mFileRef, &chain, &refNum );
		
		SetResLoad( false );
		mResFileRef = FSOpenResFile( &mFileRef, inPerm );
		SetResLoad( true );
		
		if ( kResFileNotOpened == mResFileRef )
			return ResError();
		
		// JWW - This is already known from the FSResourceFileAlreadyOpen call
		if ( alreadyOpen )
			{
			mResFileRef = kResFileNotOpened;
			return opWrErr;
			}
		}
	
	outRefNum = mResFileRef;
	return error;
}
#endif // 0


/*
**	StFileAccess::CloseAllFileRefs()
**	make sure that all open file references are closed
*/
void
StFileAccess::CloseAllFileRefs()
{
	// data fork
	if ( mFSDataForkRef != 0 )
		{
		FSCloseFork( mFSDataForkRef );
		mFSDataForkRef = 0;
		}
	
	// raw resource fork
	if ( mFSResForkRef != 0 )
		{
		FSCloseFork( mFSResForkRef );
		mFSResForkRef = 0;
		}
	
#if 0
	// resource manager access pathway
	if ( mResFileRef != kResFileNotOpened )
		{
		CloseResFile( mResFileRef );
		mResFileRef = kResFileNotOpened;
		}
#endif
}

#pragma mark -


// -----------------------------------------------------------------------------------

/*
**	FSReadOffset()
**	read some bytes from the file, with optional cache-behavior hint
*/
static OSStatus
FSReadOffset( SInt16	inFileRef,
			  UInt32 *	inOutCount,
			  void *	inDest,
			  UInt32	inOffset,
			  UInt32	inCacheFlags = 0 )
{
	// paranoia
	if ( inFileRef <= 0 || not inDest || not inOutCount )
		return paramErr;
	
	UInt16 mode = fsFromStart;
	
	//  if the user specified cache settings, use them
	if ( inCacheFlags & pleaseCacheMask )
		mode |= pleaseCacheMask;
	else
	if ( inCacheFlags & noCacheMask )
		mode |= noCacheMask;
	
	OSStatus result = FSReadFork( inFileRef,
				mode,
				inOffset,
				*inOutCount,
				inDest,
				inOutCount );
	
	return result;
}


// -----------------------------------------------------------------------------------
/*
**	FSWriteOffset()
**	write some bytes into the file, with optional caching hints
*/
static OSStatus
FSWriteOffset( SInt32		inFileRef,
			   UInt32 *		inOutCount,
			   const void *	inData,
			   UInt32		inOffset,
			   UInt32		inCacheFlags = 0 )
{
	// paranoia
	if ( inFileRef <= 0 || not inData || not inOutCount )
		return paramErr;

	UInt16 mode = fsFromStart;
	
	//  If the user specified cache settings, use them
	if ( inCacheFlags & pleaseCacheMask )
		mode |= pleaseCacheMask;
	else
	if ( inCacheFlags & noCacheMask )
		mode |= noCacheMask;
	
	OSStatus result = FSWriteFork( inFileRef, mode, inOffset, *inOutCount,inData, inOutCount );
	
	return result;
}


// JWW - The following routines were based on some functions from More Files
// The original routines were BumpDate and FSpBumpDate (thanks Jim!)

/*
**	FSSetOtherInfo()
**	
**	Given a file or directory, set its creation and modification dates.
**	Optionally also set its extended finder flags and script fields.
*/
static OSStatus
FSSetOtherInfo( const FSRef *	ref,
				UInt32			createDate,
				UInt32			modifiedDate,
				bool			useExtendedInfo,
				UInt8			theScript,
				UInt8			theExtendedFlags )
{
	FSCatalogInfo info;
	FSCatalogInfoBitmap whichInfo = kFSCatInfoCreateDate | kFSCatInfoContentMod;
	if ( useExtendedInfo )
		whichInfo |= kFSCatInfoFinderXInfo | kFSCatInfoTextEncoding;
	
	// read pre-existing information
	OSStatus error = FSGetCatalogInfo( ref,
						whichInfo,
						&info,
						nullptr, nullptr, nullptr );
	if ( noErr == error )
		{
		// twiddle the fields we care about
		LocalDateTime tmp = { 0, createDate, 0 };
		/* error = */ ConvertLocalToUTCDateTime( &tmp, &info.createDate );
		tmp.lowSeconds = modifiedDate;
		/* error = */ ConvertLocalToUTCDateTime( &tmp, &info.contentModDate );
		
		if ( useExtendedInfo )
			{
			ExtendedFileInfo& xinfo = * (ExtendedFileInfo *) &info.extFinderInfo;
			xinfo.extendedFinderFlags = theExtendedFlags;
			/* error = */ UpgradeScriptInfoToTextEncoding( theScript,
							0,			// lang
							0,			// region
							nullptr,	// fontname
							&info.textEncodingHint );
			}
		
		// write the changed info back to disk
		error = FSSetCatalogInfo( ref, whichInfo, &info );
		}
	
	return error;
}


/*****************************************************************************/

// [for Clan Lord]: Let's skip any file comments, because
//	a. FSpDTSetComment() doesn't work especially well with OS X
//	b. The files we'll be working on don't generally have (or need) comments anyway
#define IGNORE_DTDB_COMMENTS	1


#if ! IGNORE_DTDB_COMMENTS
/*
**	DTOpen()
**	Open the desktop database on a given volume
**	... not particularly OSX-savvy
*/
static OSStatus
DTOpen( ConstStr255Param volName,
	   short	vRefNum,
	   short *	dtRefNum,
	   bool *	newDTDatabase )
{
	// CL note: I removed the bits about checking if the volume supported
	// a desktop database, out of sheer laziness & not wanting to import all
	// of MoreFiles into the Clan Lord client.
	
	DTPBRec pb;
	pb.ioNamePtr = const_cast<uchar *>( volName );
	pb.ioVRefNum = vRefNum;
	OSStatus error = PBDTOpenInform( &pb );
	
	// PBDTOpenInform informs us if the desktop was just created
	// by leaving the low bit of ioTagInfo clear (0)
	* newDTDatabase = ( (pb.ioTagInfo & 1) == 0 );
	if ( paramErr == error )
		{
		error = PBDTGetPath( &pb );
		// PBDTGetPath doesn't tell us if the database is new
		// so assume it is not new
		* newDTDatabase = false;
		}
	*dtRefNum = pb.ioDTRefNum;
	return error;
}


// -----------------------------------------------------------------------------------
/*
**	FSpDTSetComment()
**	sets the OS 7+ Finder file-comment for a given file
*/
static OSStatus
FSpDTSetComment( const FSSpec * spec, ConstStr255Param comment )
{
	DTPBRec pb;
	short dtRefNum;
	bool newDTDatabase;
	
	OSStatus error = DTOpen( spec->name, spec->vRefNum, &dtRefNum, &newDTDatabase );
	if ( noErr == error )
		{
		pb.ioDTRefNum	= dtRefNum;
		pb.ioNamePtr	= const_cast<uchar *>( spec->name );
		pb.ioDirID		= spec->parID;
		pb.ioDTBuffer	= (Ptr) &comment[1];
		
		// Truncate the comment to 200 characters just in case
		// some file system doesn't range check
		if ( StrLength( comment ) <= 200 )
			pb.ioDTReqCount = StrLength( comment );
		else
			pb.ioDTReqCount = 200;
		error = PBDTSetCommentSync( &pb );
		}
	return error;
}
#endif	// ! IGNORE_DTDB_COMMENTS


/*****************************************************************************/

// CRC 16 table lookup array
static const UInt16
CRC16Table[256] = {
	0x0000, 0x1021, 0x2042, 0x3063, 0x4084, 0x50A5, 0x60C6, 0x70E7,
	0x8108, 0x9129, 0xA14A, 0xB16B, 0xC18C, 0xD1AD, 0xE1CE, 0xF1EF,
	0x1231, 0x0210, 0x3273, 0x2252, 0x52B5, 0x4294, 0x72F7, 0x62D6,
	0x9339, 0x8318, 0xB37B, 0xA35A, 0xD3BD, 0xC39C, 0xF3FF, 0xE3DE,
	0x2462, 0x3443, 0x0420, 0x1401, 0x64E6, 0x74C7, 0x44A4, 0x5485,
	0xA56A, 0xB54B, 0x8528, 0x9509, 0xE5EE, 0xF5CF, 0xC5AC, 0xD58D,
	0x3653, 0x2672, 0x1611, 0x0630, 0x76D7, 0x66F6, 0x5695, 0x46B4,
	0xB75B, 0xA77A, 0x9719, 0x8738, 0xF7DF, 0xE7FE, 0xD79D, 0xC7BC,
	0x48C4, 0x58E5, 0x6886, 0x78A7, 0x0840, 0x1861, 0x2802, 0x3823,
	0xC9CC, 0xD9ED, 0xE98E, 0xF9AF, 0x8948, 0x9969, 0xA90A, 0xB92B,
	0x5AF5, 0x4AD4, 0x7AB7, 0x6A96, 0x1A71, 0x0A50, 0x3A33, 0x2A12,
	0xDBFD, 0xCBDC, 0xFBBF, 0xEB9E, 0x9B79, 0x8B58, 0xBB3B, 0xAB1A,
	0x6CA6, 0x7C87, 0x4CE4, 0x5CC5, 0x2C22, 0x3C03, 0x0C60, 0x1C41,
	0xEDAE, 0xFD8F, 0xCDEC, 0xDDCD, 0xAD2A, 0xBD0B, 0x8D68, 0x9D49,
	0x7E97, 0x6EB6, 0x5ED5, 0x4EF4, 0x3E13, 0x2E32, 0x1E51, 0x0E70,
	0xFF9F, 0xEFBE, 0xDFDD, 0xCFFC, 0xBF1B, 0xAF3A, 0x9F59, 0x8F78,
	0x9188, 0x81A9, 0xB1CA, 0xA1EB, 0xD10C, 0xC12D, 0xF14E, 0xE16F,
	0x1080, 0x00A1, 0x30C2, 0x20E3, 0x5004, 0x4025, 0x7046, 0x6067,
	0x83B9, 0x9398, 0xA3FB, 0xB3DA, 0xC33D, 0xD31C, 0xE37F, 0xF35E,
	0x02B1, 0x1290, 0x22F3, 0x32D2, 0x4235, 0x5214, 0x6277, 0x7256,
	0xB5EA, 0xA5CB, 0x95A8, 0x8589, 0xF56E, 0xE54F, 0xD52C, 0xC50D,
	0x34E2, 0x24C3, 0x14A0, 0x0481, 0x7466, 0x6447, 0x5424, 0x4405,
	0xA7DB, 0xB7FA, 0x8799, 0x97B8, 0xE75F, 0xF77E, 0xC71D, 0xD73C,
	0x26D3, 0x36F2, 0x0691, 0x16B0, 0x6657, 0x7676, 0x4615, 0x5634,
	0xD94C, 0xC96D, 0xF90E, 0xE92F, 0x99C8, 0x89E9, 0xB98A, 0xA9AB,
	0x5844, 0x4865, 0x7806, 0x6827, 0x18C0, 0x08E1, 0x3882, 0x28A3,
	0xCB7D, 0xDB5C, 0xEB3F, 0xFB1E, 0x8BF9, 0x9BD8, 0xABBB, 0xBB9A,
	0x4A75, 0x5A54, 0x6A37, 0x7A16, 0x0AF1, 0x1AD0, 0x2AB3, 0x3A92,
	0xFD2E, 0xED0F, 0xDD6C, 0xCD4D, 0xBDAA, 0xAD8B, 0x9DE8, 0x8DC9,
	0x7C26, 0x6C07, 0x5C64, 0x4C45, 0x3CA2, 0x2C83, 0x1CE0, 0x0CC1,
	0xEF1F, 0xFF3E, 0xCF5D, 0xDF7C, 0xAF9B, 0xBFBA, 0x8FD9, 0x9FF8,
	0x6E17, 0x7E36, 0x4E55, 0x5E74, 0x2E93, 0x3EB2, 0x0ED1, 0x1EF0
};


// CalculateCRC -- Excerpted from my Shrink II source code from years back...
static UInt16
CalculateCRC( UInt16 CRC, const char * dataBlock, UInt32 dataSize )
{	
	while ( dataSize )
		{
		CRC = (CRC << 8) ^ CRC16Table[((*dataBlock) ^ (CRC >> 8)) & 0x00FF];
		++dataBlock;
		--dataSize;
		}
	
	return CRC;
}


/*
**	IsValidMacBinary()
**	This tests for a valid MacBinary, MacBinary II, or MacBinary III header
*/
bool
IsValidMacBinary( const char * theHeader, MacHeaderVersion * theVersion )
{
	bool isValid = false;
	MacHeaderVersion internalVersion = kMacBinary;
	
	/*
	MacBinary, MacBinary II, and MacBinary III have the following minimum requirements:
		MBOldVersion must be zero
		MBZeroFill1 must be zero
		MBZeroFill2 must be zero
		MBComputerOS must be zero
		MBFilenameLength must be between 1 and 63
		
		MacBinary III restricts MBFilenameLength to 31, but that's checked later on
	*/
	
	if ( MBOldVersion(theHeader) == 0
	&&   MBZeroFill1(theHeader) == 0
	&&   MBZeroFill2(theHeader) == 0
	&&   MBComputerOS(theHeader) == 0
	&&   MBFilenameLength(theHeader) >= 1
	&&   MBFilenameLength(theHeader) <= 63 )
		{
			/*
			MacBinary II and MacBinary III have the following requirements:
				MBSecondaryHeader must be zero
				MBMinVersion must be kVersion2 (129)
				MBChecksum must match the CRC of bytes 0 to kChecksumEnd (124)
			*/
		
		if ( 0 == MBSecondaryHeader(theHeader)
		&&   kVersion2 == MBMinVersion(theHeader)
		&&   CalculateCRC( 0, theHeader, kChecksumEnd ) == MBChecksum(theHeader) )
			{
			// Check for the specific MacBinary II version number
			if ( kVersion2 == MBVersion2(theHeader) )
				{
				isValid = true;
				internalVersion = kMacBinaryII;
				}
			
				/*
					MacBinary III has the following requirements:
					MBVersion2 must be kVersion3 (130)
					MBSignature must be kMacBinarySignature ('mBIN')
					MBFilenameLength must be between 1 and 31
				*/
			
			// Check for the specific MacBinary III version and tags
			else
			if ( kVersion3 == MBVersion2(theHeader)
			&&   kMacBinarySignature == MBSignature(theHeader)
			&&   MBFilenameLength(theHeader) <= 31 )
				{
				isValid = true;
				internalVersion = kMacBinaryIII;
				}
			}
		
		if ( kMacBinary == internalVersion )
			{
				/*
					MacBinary 1 has the following additional requirements:
					MBDataFork     must not be bigger than 0x007FFFFF
					MBResourceFork must not be bigger than 0x007FFFFF
					
					Unused bytes in the header must also be zero
				*/
			
			// Test for valid ranges in the data fork and resource fork sizes
			if ( not ( (MBDataFork(theHeader) & ~0x007FFFFF)
			      && (MBResourceFork(theHeader) & ~0x007FFFFF)) )
				isValid = true;
			
			// Test for the large field of zeros
			int i = 0;
			while ( isValid && i < kVersion1ZeroFillLength )
				{
				if ( MBFindByte( theHeader, kVersion1ZeroFill + i ) != 0 )
					isValid = false;
				
				++i;
				}
			}
		}
	
	// return optional version number
	if ( theVersion )
		*theVersion = internalVersion;
	
	return isValid;
}


#define ThrowIfErr( err ) do { if ( err ) throw OSStatus( err ); } while (0)


/*
**	FSDecodeMacBinary()
**	HFS+ version
*/
OSStatus
FSDecodeMacBinary( const FSRef * inTargetRef,
				OSType inCreator, OSType inType, ScriptCode inScript )
{
	__Check( inTargetRef );
	if ( nullptr == inTargetRef )
		return paramErr;
	
	OSStatus theErr;
	bool isCreated = false;
	bool isRenamed = false;
	FSRef sourceRef, targetRef, parDirRef;
	char * theBuffer = nullptr;
	HFSUniStr255 origName;
	
	try
		{
		// grab some working storage
		theBuffer = new char[ kDefaultBufferSize ];
		
		//  Rename the file we're trying to decode to something squirrely...
		UniChar tempName[ 32 ];
		int tempNameLen = 4;
		
		char tmpBuff[ 32 ];
		std::snprintf( tmpBuff, sizeof tmpBuff, "tmp_%lu", TickCount() );
		for ( const char * p = tmpBuff; *p; ++p )
			tempName[ tempNameLen++ ] = UniChar( *p );
		
		// get original name (as HFSUniStr); also the parent dir (as FSRef)
		theErr = FSGetCatalogInfo( inTargetRef, kFSCatInfoNone, nullptr, &origName,
									nullptr, &parDirRef );
		ThrowIfErr( theErr );
		HFSUniStr255 dstName = origName;
		
		// erase any leftovers -- although there shouldn't be any, due to the funky name
		FSRef tempref;
		theErr = FSMakeFSRefUnicode( &parDirRef,
						tempNameLen, tempName,
						kTextEncodingMacRoman, &tempref );
		if ( fnfErr == theErr )
			theErr = noErr;
		ThrowIfErr( theErr );
		
		theErr = FSDeleteObject( &tempref );
		if ( nsvErr == theErr )
			theErr = noErr;
		ThrowIfErr( theErr );
		
		// rename orig to "tmp_XXX"
		theErr = FSRenameUnicode( inTargetRef,
						tempNameLen, tempName,
						kTextEncodingMacRoman,	// it's just "tmp_" and some digits, hence ASCII
						&sourceRef );
		ThrowIfErr( theErr );
		
		isRenamed = true;
		
		// trim off the trailing ".bin" extension, if any
		{
		const UniChar * trail = dstName.unicode + dstName.length - 4;
		if ( '.' == trail[0]
		&&   ('b' == trail[1] || 'B' == trail[1])
		&&   ('i' == trail[2] || 'I' == trail[2])
		&&   ('n' == trail[3] || 'N' == trail[3]) )
			{
			dstName.length -= 4;
			
			// remove previous leftovers (which are marginally more likely,
			// because the non-encoded name isn't "squirrely")
			// In general this might be overambitious, but for the CLClient's purpose, we're OK
			// because this is all happening in the Downloads folder, where we can resolve
			// name collisions in any manner we see fit, no matter how drastic.
			theErr = FSMakeFSRefUnicode( &parDirRef,
							dstName.length, dstName.unicode,
							kTextEncodingUnknown,
							&targetRef );
			if ( fnfErr == theErr )
				theErr = noErr;
			else
			if ( noErr == theErr )
				theErr = FSDeleteObject( &targetRef );
			ThrowIfErr( theErr );
			}
		}
		
		FSCatalogInfo info;
		FileInfo& finfop = * (FileInfo *) &info.finderInfo;
		finfop.fileCreator = inCreator;
		finfop.fileType = inType;
		theErr = UpgradeScriptInfoToTextEncoding( inScript,
						0,		// lang code?
						0,		// region code?
						nullptr,	// font name
						&info.textEncodingHint );
		ThrowIfErr( theErr );
		theErr = FSCreateFileUnicode( &parDirRef,
						dstName.length,
						dstName.unicode,
						kFSCatInfoFinderInfo | kFSCatInfoTextEncoding,
						&info,
						&targetRef,
						nullptr );
		ThrowIfErr(theErr);
		isCreated = true;
		
		//  Open both file forks for locking from other threads.
		//  Scope is important here because StFileAccess will close files when destroyed.
		{
			StFileAccess targetItemAccess( targetRef );
			StFileAccess sourceItemAccess( sourceRef );
			
			// assume decoding will fail, and arrange for automatic cleanup of leftovers
			targetItemAccess.SetDelete( true );
			
			SInt16 sourceDataRef;
			theErr = sourceItemAccess.GetFSDataForkRef( fsRdWrPerm, sourceDataRef );
			ThrowIfErr(theErr);
			SInt16 sourceResRef;
			theErr = sourceItemAccess.GetFSResForkRef( fsRdWrPerm, sourceResRef );
			ThrowIfErr(theErr);
			
			SInt16 targetDataRef;
			theErr = targetItemAccess.GetFSDataForkRef( fsRdWrPerm, targetDataRef );
			ThrowIfErr(theErr);
			SInt16 targetResRef;
			theErr = targetItemAccess.GetFSResForkRef( fsRdWrPerm, targetResRef );
			ThrowIfErr(theErr);
			
			// if the input file is smaller than a MacBinary header, it can't possibly be decodable
			SInt64 maxOffset;
			theErr = FSGetForkSize( sourceDataRef, &maxOffset );
			ThrowIfErr(theErr);
			
			if ( maxOffset < kMacBinaryHeaderSize )
				{
				theErr = paramErr;
				throw OSStatus( paramErr );
				}
			
			// read the header; see if it's sane
			char theHeader[ kMacBinaryHeaderSize ];
			UInt32 requestCount = sizeof theHeader;
			theErr = FSReadOffset( sourceDataRef, &requestCount, theHeader, 0, noCacheMask );
			ThrowIfErr(theErr);
			
			MacHeaderVersion theVersion;
			if ( not IsValidMacBinary( theHeader, &theVersion ) )
				{
				theErr = kNotMacBinaryErr;
				ThrowIfErr(theErr);
				}
			
			// extract & copy the data fork
			UInt32 remaining = MBDataFork( theHeader );
			UInt32 currentSourcePos =
				(kMacBinaryHeaderSize + MBSecondaryHeader(theHeader) + 0x7F) & ~0x7F;
			UInt32 currentTargetPos = 0;
			
			while ( remaining )
				{
				if ( remaining > kDefaultBufferSize )
					requestCount = kDefaultBufferSize;
				else
					requestCount = remaining;
				
				theErr = FSReadOffset( sourceDataRef, &requestCount, theBuffer,
							currentSourcePos, noCacheMask );
				ThrowIfErr(theErr);
				
				theErr = FSWriteOffset( targetDataRef, &requestCount,
							theBuffer, currentTargetPos, noCacheMask );
				ThrowIfErr(theErr);
				
				remaining -= requestCount;
				currentSourcePos += requestCount;
				currentTargetPos += requestCount;
				}
			
			// extract & copy the (raw) resource fork
			remaining = MBResourceFork(theHeader);
			currentSourcePos = (currentSourcePos + 0x7F) & ~0x7F;
			currentTargetPos = 0;
			
			while ( remaining )
				{
				if ( remaining > kDefaultBufferSize )
					requestCount = kDefaultBufferSize;
				else
					requestCount = remaining;
				
				theErr = FSReadOffset( sourceDataRef, &requestCount, theBuffer,
							currentSourcePos, noCacheMask );
				ThrowIfErr(theErr);
				
				theErr = FSWriteOffset( targetResRef, &requestCount, theBuffer,
							currentTargetPos, noCacheMask );
				ThrowIfErr(theErr);
				
				remaining -= requestCount;
				currentSourcePos += requestCount;
				currentTargetPos += requestCount;
				}
			
			remaining = MBComment( theHeader );
			currentSourcePos = (currentSourcePos + 0x7F) & ~0x7F;
			
			// The comment field did not exist until MacBinary II
			if ( remaining && (theVersion != kMacBinary) )
				{
				// This works because comments cannot be longer than 200 characters and the
				// buffer size should be much (much much) larger than that
				if ( remaining > kDefaultBufferSize )
					requestCount = kDefaultBufferSize;
				else
					requestCount = remaining;
				
				// Read the comment to theBuffer[1] so we can set theBuffer[0] to the length
				theErr = FSReadOffset( sourceDataRef, &requestCount,
							theBuffer + 1, currentSourcePos, noCacheMask );
				ThrowIfErr(theErr);
				
				// FSpDTSetComment expects a pascal string for the comment, so set the length byte
				theBuffer[0] = requestCount;
				
#if ! IGNORE_DTDB_COMMENTS
				theErr = FSpDTSetComment( &targetSpec, (ConstStr255Param) theBuffer );
				ThrowIfErr(theErr);
#endif // ! IGNORE_DTDB_COMMENTS
				
				currentSourcePos += requestCount;
				}
			
			finfop.fileType = MBFiletype( theHeader );
			finfop.fileCreator = MBCreator( theHeader );
			
			// The low byte of the finderFlags did not exist until MacBinary II
			if ( kMacBinary == theVersion )
				finfop.finderFlags = MBFinderFlagsHigh(theHeader) << 8;
			else
				finfop.finderFlags = MBFinderFlags( theHeader );
			
			// These bits should always be clear when creating a new file
			finfop.finderFlags &= ~(kIsOnDesk | kHasBeenInited);
			
			finfop.location.v = MBVerticalPos(theHeader);
			finfop.location.h = MBHorizontalPos(theHeader);
			
			theErr = FSSetCatalogInfo( &targetRef, kFSCatInfoFinderInfo, &info );
			ThrowIfErr(theErr);
			
			// Do this so the modification date isn't mis-set by closing the file
			targetItemAccess.CloseAllFileRefs();
			
			// The script and extended Finder flags did not exist until MacBinary III
			bool useExtendedFlags = ( kMacBinaryIII == theVersion );
			
			theErr = FSSetOtherInfo( &targetRef,
						MBCreatedDate(theHeader), MBModifiedDate(theHeader),
						useExtendedFlags, MBScript(theHeader), MBExtendedFlags(theHeader) );
			ThrowIfErr(theErr);
			
			// if we got here, the file has been completely & successfully decoded:
			// so don't garbage-collect it
			targetItemAccess.SetDelete( false );
			}
		
		FSDeleteObject( &sourceRef );
		}
	
	catch ( const OSStatus& inException )
		{
		theErr = inException;
		
		if ( isCreated )
			FSDeleteObject( &targetRef );
		
		if ( isRenamed )
			FSRenameUnicode( &sourceRef,
					origName.length,
					origName.unicode,
					kTextEncodingUnknown,
					nullptr );
		}
	catch (...)
		{
		theErr = paramErr;
		
		if ( isCreated )
			FSDeleteObject( &targetRef );
		
		if ( isRenamed )
			FSRenameUnicode( &sourceRef,
					origName.length,
					origName.unicode,
					kTextEncodingUnknown,
					nullptr );
		}
	
	delete[] theBuffer;
	
	return theErr;
}

