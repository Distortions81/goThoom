/*
**	Memory_mac.cp		dtslib2
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

#include "Memory_cmn.h"
#include "Shell_mac.h"

/*
**	Entry Routines
**
**	void *	PlatformNewAlloc( size_t size ) DOES_NOT_THROW;
**	void	PlatformNewFree( void * ptr ) DOES_NOT_THROW;
**	void	PlatformNewErrMsg( const char * msg ) DOES_NOT_THROW;
*/


/*
**	Internal Routines
*/
#if TARGET_API_MAC_OS8
static long MyGrowZone( long size );
#endif


/*
**	PlatformNewAlloc()
**
**	get some memory from the operating system
**	juggle the grow zone proc and call NewPtr()
*/
void *
PlatformNewAlloc( size_t size ) DOES_NOT_THROW
{
#if TARGET_API_MAC_OS8
	// remove our grow zone proc
	// if the newptr fails the new handler will be called anyway
	::SetGrowZone( nullptr );
#endif
	
	void * ptr = nullptr;
	try
		{
		ptr = ::NewPtr( size );
		}
	catch (...) {}
	
#if TARGET_API_MAC_OS8
	// (re-)install our grow zone proc
	static GrowZoneUPP upp;
	if ( not upp )
		{
		upp = ::NewGrowZoneUPP( MyGrowZone );
		__Check( upp );
		}
	::SetGrowZone( upp );
#endif
	
	return ptr;
}


/*
**	PlatformNewFree()
**
**	return unused memory to the operating system
**	call DisposePtr()
*/
void
PlatformNewFree( void * ptr ) DOES_NOT_THROW
{
	try
		{
		::DisposePtr( (Ptr) ptr );
		}
	catch (...) {}
}


/*
**	PlatformNewErrMsg()
**
**	call VDebugStr()
*/
void
PlatformNewErrMsg( const char * msg ) DOES_NOT_THROW
{
	try
		{
		VDebugStr( "%s", msg );
		}
	catch (...) {}
}


#if TARGET_API_MAC_OS8
/*
**	MyGrowZone()
**
**	custom grow zone procedure
*/
long
MyGrowZone( long /*sizeneeded*/ )
{
	// call the new handler until a pool is freed
	// or the new handler gives up
	bPoolFreed = false;
	for (;;)
		{
		// get current new-handler
		std::new_handler nh = std::set_new_handler( nullptr );
		
		// if none set, we're hosed
		if ( not nh )
			break;
		
		// re-install it, and call it
		std::set_new_handler( nh );
		nh();
		
		if ( bPoolFreed )
			return 1;	// a non-zero number indicating success
		}
	
	// oh deer
	// we're likely to be in trouble now
	return 0;	// failure
}
#endif  // TARGET_API_MAC_OS8

