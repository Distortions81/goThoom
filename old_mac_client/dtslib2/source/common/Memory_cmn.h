/*
**	Memory_cmn.h		dtslib2
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

#ifndef Memory_cmn_h
#define Memory_cmn_h

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "Local_dts.h"


/*
**	Required Platform Dependent routines
*/
void *	PlatformNewAlloc( size_t size ) DOES_NOT_THROW;
void	PlatformNewFree( void * ptr ) DOES_NOT_THROW;
void	PlatformNewErrMsg( const char * msg ) DOES_NOT_THROW;


/*
**	Variables
*/
extern bool bPoolFreed;	// might be used by PlatformNewAlloc.


#endif // Memory_cmn_h
