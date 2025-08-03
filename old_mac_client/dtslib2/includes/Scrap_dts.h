/*
**	Scrap_dts.h		dtslib2
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

#ifndef Scrap_dts_h
#define Scrap_dts_h

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "Local_dts.h"


/*
**	Interface routines
*/
				// removes all data from the scrap
DTSError	ClearScrap();

				// copy data to the scrap
DTSError	CopyToScrap( ulong stype, const void * ptr, size_t sz );

				// get the size of the data in the scrap
size_t		GetScrapSize( ulong stype );

				// get data from the scrap
DTSError	GetFromScrap( ulong stype, void * ptr, size_t dstSize = 0 );


#endif  // Scrap_dts_h
