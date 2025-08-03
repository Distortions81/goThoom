/*
**	Memory_dts.h		dtslib2
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
**
**
**	This file declares the outside interface to the low-level memory
**	allocator, specifically the leak-tracking routines. See New_dts.h
**	for the high-level interface, i.e. custom operator new & delete.
*/

#ifndef Memory_dts_h
#define Memory_dts_h

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif


// DEBUG_VERSION_TAGS implies DEBUG_VERSION_NEW
#if defined( DEBUG_VERSION_TAGS ) && DEBUG_VERSION_TAGS
# if ! defined( DEBUG_VERSION_NEW ) || DEBUG_VERSION_NEW == 0
#  define DEBUG_VERSION_NEW		1
# endif
#endif


/*
**	leak tracking routines
*/
#ifdef DEBUG_VERSION_TAGS
void	DumpBlocks( const char * filename );
void	DumpBlocksSummary( const char * filename );
#endif


#endif	// Memory_dts_h
