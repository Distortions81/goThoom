/*
**	Frame_cl.cp		ClanLord
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

#include "Frame_cl.h"

#include "ClanLord.h"
#include "Movie_cl.h"

/*
**	CCLFrame::CCLFrame()
**	ctor -- zero it out
*/
CCLFrame::CCLFrame()
{
	Reset();
}


/*
**	CCLFrame::Reset()
**	initialize the frame data
*/
void
CCLFrame::Reset()
{
	mFrameLen		= 0;
#ifdef DEBUG_VERSION
	mExtraLen		= 0;
	mDescLen		= 0;
	mPictLen		= 0;
	mMobileLen		= 0;
#endif
	mAckCmdNum		= 0;
	mAckFrame		= 0;
	mResentFrame	= 0;
	mNumDesc		= 0;
	mNumPict		= 0;
	mNumPictAgain	= 0;
	mNumMobile		= 0;
	mStateLen		= 0;
	mLightFlags		= 0;
	mHP	= mHPMax = mSP = mSPMax = mBalance = mBalanceMax = 255;
}


/*
**	CCLFrame::ReadKeyFromSpool()
**	extract data from spool file
*/
void
CCLFrame::ReadKeyFromSpool( DataSpool * inSpool )
{
	int startMark	= inSpool->GetMark();
	
#ifdef DEBUG_VERSION
	// initialize frame stats
	mExtraLen		= 0;
	mDescLen		= 0;
	mPictLen		= 0;
	mMobileLen		= 0;
#endif	// DEBUG_VERSION
	
	mFrameLen		= 0;
	
	mAckCmdNum 		= inSpool->GetNumber( kSpoolUnsignedByte );
	mAckFrame 		= inSpool->GetNumber( kSpoolUnsignedLong );
	mResentFrame 	= inSpool->GetNumber( kSpoolUnsignedLong );
	
#ifdef DEBUG_VERSION
	int tempMark	= inSpool->GetMark();
#endif	// DEBUG_VERSION
	
	// extract descriptors
	mNumDesc		= inSpool->GetNumber( kSpoolUnsignedByte );
	if ( mNumDesc > max_Desc )
		{
		ShowMessage( BULLET " HELP: %d descriptors (%d max)", (int) mNumDesc, max_Desc );
		mNumDesc		= 0;
		mFrameLen		= inSpool->GetMark() - startMark;
		return;
		}
	for ( uint i = 0; i < mNumDesc;  ++i )
		{
		SFrameDesc&		desc = mDesc[i];
		
		desc.mIndex		= inSpool->GetNumber( kSpoolUnsignedByte );
		desc.mType		= inSpool->GetNumber( kSpoolUnsignedByte );
		desc.mPictID	= inSpool->GetNumber( kSpoolUnsignedShort );
		
		// special case for pictID -1
		// which, unlike children, should be heard but not seen.
		if ( desc.mPictID == 0xFFFF )
			{
/*
	It doesn't really matter what pictID is substituted for -1 here.
	We're just trying to prevent the data-management cache from requesting
	PDef 0xFFFF from the server, and (in rare cases) the players window from
	drawing a pictureless player.
	Proving the above assertion is left as an exercise to the reader.
	Let's use the boat picture as a convenient placeholder, because it has
	beneficial properties vis-a-vis the player window.
*/
			desc.mPictID = 537; // boat picture
			}
		
		inSpool->GetString( desc.mName, sizeof desc.mName );
		desc.mNameLen	= std::strlen( desc.mName );
		
		desc.mNumColors	= inSpool->GetNumber( kSpoolUnsignedByte );
		inSpool->GetData( desc.mColors, desc.mNumColors );
		}
#ifdef DEBUG_VERSION
	mDescLen		= inSpool->GetMark() - tempMark;
#endif	// DEBUG_VERSION
	
	// extract 'me' information
	mHP				= inSpool->GetNumber( kSpoolUnsignedByte );
	mHPMax			= inSpool->GetNumber( kSpoolUnsignedByte );
	mSP				= inSpool->GetNumber( kSpoolUnsignedByte );
	mSPMax			= inSpool->GetNumber( kSpoolUnsignedByte );
	mBalance		= inSpool->GetNumber( kSpoolUnsignedByte );
	mBalanceMax		= inSpool->GetNumber( kSpoolUnsignedByte );
	mLightFlags		= inSpool->GetNumber( kSpoolUnsignedByte );
	
#ifdef DEBUG_VERSION
	tempMark		= inSpool->GetMark();
#endif	// DEBUG_VERSION
	
	// extract pictures
	mNumPict		= inSpool->GetNumber( kSpoolUnsignedByte );
	mNumPictAgain   = 0;
	// check for the magic value
	if ( mNumPict == 255 )
		{
		// this is the number of old (permanent-like) pictures to draw again from last frame
		mNumPictAgain = inSpool->GetNumber( kSpoolUnsignedByte );
		// this is the number of new (temporary-like) pictures to draw in addition this frame
		mNumPict      = inSpool->GetNumber( kSpoolUnsignedByte );
		}
	if ( mNumPict + mNumPictAgain > max_Picture )
		{
		ShowMessage( BULLET " HELP: %d pictures (%d max)",
			(int) mNumPict + mNumPictAgain, max_Picture );
		mNumPict		= 0;
		mNumPictAgain   = 0;
		mFrameLen		= inSpool->GetMark() - startMark;
		return;
		}
	for ( uint i = 0; i < mNumPict; ++i )
		{
		SFramePict&		pict = mPict[ mNumPictAgain + i ];
		
		DTSKeyID pictID;
		int horz, vert;

#if BITWISE_IMAGE_SPOOL
		if ( CCLMovie::IsReading() )
			{
			// let the movie figure out what format to use
			CCLMovie::InterpretFramePicture( inSpool, pictID, horz, vert );
			}
		else
#endif	// BITWISE_IMAGE_SPOOL
			UnspoolFramePicture( inSpool, pictID, horz, vert );
		
		pict.mPictID	= pictID;
		pict.mH			= horz;
		pict.mV			= vert;
		
#ifdef DEBUG_IMAGECOUNT
		// keep track of how often each image is received
		if ( pictID >= 0 && pictID < kPictFrameIDLimit )
			gImageCounts[ pictID ] += 1;
		else
			ShowMessage( "received image %d!", (int) pictID );
#endif // DEBUG_IMAGECOUNT
		}

// realign: move to start of next byte if needed
#if BITWISE_IMAGE_SPOOL
	if ( mNumPict )
		inSpool->ByteAlignRead();
#endif	// BITWISE_IMAGE_SPOOL
	
#ifdef DEBUG_VERSION
	mPictLen		= inSpool->GetMark() - tempMark;
	tempMark		= inSpool->GetMark();
#endif	// DEBUG_VERSION
	
	// extract mobiles
	mNumMobile		= inSpool->GetNumber( kSpoolUnsignedByte );
	if ( mNumMobile > max_Mobile )
		{
		ShowMessage( BULLET " HELP: %d mobiles (%d max)", (int) mNumMobile, max_Mobile );
		mNumMobile		= 0;
		mFrameLen		= inSpool->GetMark() - startMark;
		return;
		}
	for ( uint i = 0; i < mNumMobile; ++i )
		{
		SFrameMobile& 	mobile = mMobile[i];
		
		mobile.mIndex	= inSpool->GetNumber( kSpoolUnsignedByte );
		mobile.mState	= inSpool->GetNumber( kSpoolUnsignedByte );
		mobile.mH		= inSpool->GetNumber( kSpoolSignedShort  );
		mobile.mV		= inSpool->GetNumber( kSpoolSignedShort  );
		mobile.mColors	= inSpool->GetNumber( kSpoolUnsignedByte );
		}
#ifdef DEBUG_VERSION
	mMobileLen		= inSpool->GetMark() - tempMark;
#endif	// DEBUG_VERSION
	
	// extract State information
	mStateLen		= inSpool->GetLimit() - inSpool->GetMark();
	if ( mStateLen > size_Frame )
		{
		ShowMessage( BULLET " HELP: %d frame state (%d max)", mStateLen, size_Frame );
		mStateLen		= 0;
		mFrameLen		= inSpool->GetMark() - startMark;
		return;
		}
	if ( mStateLen )
		inSpool->GetData( mStateData, mStateLen );
	
	mFrameLen		= inSpool->GetMark() - startMark;
	
#ifdef DEBUG_VERSION
	mExtraLen		= mFrameLen - mStateLen - mMobileLen - mPictLen - mDescLen;
	// mExtraLen should always be 16:
	//	ackCmdNum:				1
	//	{ack,resent}Frame:		4 + 4
	//	{HP,SP,Bal}{Cur,Max}:	6
	//	light flags:			1
#endif	// DEBUG_VERSION
}

