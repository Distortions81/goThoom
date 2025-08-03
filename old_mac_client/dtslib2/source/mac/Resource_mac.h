/*
**	Resource_mac.h		dtslib2
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

#ifndef Resource_mac_h
#define Resource_mac_h

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif


// resource ids
enum
{
	// PICT
	rLogoPictID				= 32000,
	kNumLogoPicts			= 12,
	rAboutColorPictID		= 32100,	// yeah, these 2 are...
//	rAboutBWPictID			= 32101,	// the wrong way 'round
	rCreditsPictID			= 32103,
	
	// STR 
	rUserStrID				= -16096,
	
	// ALRT/DITL
	rGenericErrorAlrtID		= 32002,
	rGenericOkCancelAlrtID	= 32003,
	rGenericYesNoAlrtID		= 32004,
	
	// DLOG/DITL
//	rAboutBWDlgID			= 32100,
	rAboutColorDlgID		= 32101
};


#endif  // Resource_mac_h

