/*
**	Frame_cl.h		ClanLord
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

#ifndef FRAME_CL_H
#define FRAME_CL_H

#include "Public_cl.h"

//
// This class is used to unpack/pack frame data from/to the network spool
// This is an abstraction; you should never extract data directly from
// the spool.
//
class CCLFrame
{
public:
					CCLFrame();
	
	bool			IsValid()	const { return mFrameLen != 0; }
	void			Reset();
	void			ReadKeyFromSpool( DataSpool * inSpool );
	
	// for later. be patient, this bit field will get used
	enum
		{
		pack_AckCmdNum			=	0x00000001,
		pack_NumDesc			=	0x00000002,
		pack_HP					= 	0x00000004,
		pack_SP					=	0x00000008,
		pack_Balance			= 	0x00000010,
		pack_FaceDir			=	0x00000020,
		pack_Pict				= 	0x00000040,
		pack_Mobile				=	0x00000080
		};
	//
	// a bunch of sizes. it is made to be maintainable!
	// Ain't it cool?
	//
	enum
		{
		size_FrameData			= 	sizeof(Message),	// was 544, but let's allow more
		
		size_StartMin			= 	sizeof(uchar) + 	// ack number
									sizeof(int32_t) +	// ack frame
									sizeof(int32_t) +	// resent frame
									sizeof(uchar) + 	// num descriptors
									sizeof(uchar) +		// num pictures
									sizeof(uchar),		// num mobiles
		
		size_StartMax			= 	size_StartMin,
		
		size_DescMin			= 	sizeof(uchar) + 	// index
									sizeof(uchar) + 	// type
									sizeof(int16_t) + 	// pict ID
									sizeof(uchar) + 0 + // name len + name data
									sizeof(uchar) + 0,	// num colors + colors
		
		size_DescMax			= 	sizeof(uchar) + 	// index
									sizeof(uchar) + 	// type
									sizeof(int16_t) + 	// pict ID
									1+kMaxNameLen +  	// name len + name data
									1+kNumPlyColors,	// num colors + colors
		
		size_HeaderMin			= 	sizeof(uchar) * 7,	// hp, spirit, bal, lighting
		size_HeaderMax			= 	size_HeaderMin,
		
		size_MobileMin			= 	sizeof(uchar) + 	// index
									sizeof(uchar) +		// state
									sizeof(int16_t) + 	// H pos
									sizeof(int16_t) +	// V pos
									sizeof(uchar),		// color bits
		size_MobileMax			=	size_MobileMin,
		
		size_PictureMin			=	sizeof(int32_t),	// fewest bytes any version used (packed)
		size_PictureMax			= 	size_PictureMin,
		
		size_Frame				= 	size_FrameData - size_StartMin - size_HeaderMax,
		
		max_Desc				= 	size_Frame / size_DescMin,
		
		// Multiplied by two to allow for the worst case, where there are
		// the maximum number of BOTH static-like old pictures from prior frames,
		// AND dynamic-like (new) pictures from the current frame
		// More recent versions use more bits per image sent, so this overestimates
		// the number possible for those, but that many are still needed when playing old movies.
		max_Picture				=	(2 * size_Frame / size_PictureMin),
		
		max_Mobile				=	size_Frame / size_MobileMin
		};
	
	// a Descriptor, extracted from the frame
	struct SFrameDesc
		{
		uint			mIndex 		: 8,
						mType		: 8,
						mPictID		: 16;
		uint			mNameLen	: 8,
						mNumColors	: 8;
		
		char			mName[ kMaxNameLen ];		// len + name + zero
		uchar			mColors[ kNumPlyColors ];	// len + colors
		};
	
	// a Picture, extracted from frame
	struct SFramePict
		{
		int16_t			mPictID;
		int16_t			mH;
		int16_t			mV;
		};
	
	// a Mobile, extracted from the frame
	struct SFrameMobile
		{
		uint			mIndex		: 8,
						mState		: 8,
						mColors		: 8;
		int16_t			mH;
		int16_t			mV;
		};
	
	
	int32_t					mFrameLen;
#ifdef DEBUG_VERSION
	// these are used for statistical purposes
	int						mExtraLen;
	int						mDescLen;
	int						mPictLen;
	int						mMobileLen;
#endif	// DEBUG_VERSION
	
	int32_t					mAckFrame;
	int32_t					mResentFrame;
	
	uint					mHP			: 8,
							mHPMax		: 8,
							mSP			: 8,
							mSPMax		: 8;
	
	uint					mBalance	: 8,
							mBalanceMax	: 8,
							mLightFlags	: 8,
							mAckCmdNum	: 8;
	
	uint					mNumDesc		: 8,
							mNumPict		: 8,
							mNumPictAgain	: 8,
							mNumMobile		: 8;
	
	// real maximum number of descriptor, ever. just to be sure :)
	SFrameDesc				mDesc[ max_Desc ];
	SFramePict				mPict[ max_Picture ];
	SFrameMobile			mMobile[ max_Mobile ];
	
	int						mStateLen;
	uchar					mStateData[ size_Frame ];
};


// There's only one!
extern CCLFrame *		gFrame;		// from GameWin_cl.cp

#endif  // FRAME_CL_H

