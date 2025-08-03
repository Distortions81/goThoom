/*
**	New_mac.cp		dtslib2
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

#if 0
/************************************************************************/
/*	Project...:	Standard C++ Library									*/
/*	Name......:	New.cp													*/
/*	Purpose...:	standard C++ library									*/
/*  Copyright.: Copyright (c) 1993-1998 Metrowerks, Inc.				*/
/*  Portions Copyright: Copyright (c) 1999-2002 Delta Tao Software, Inc.*/
/************************************************************************/

#ifndef _dtslib2_
#include "Prefix_dts.h"
#endif

#include "Memory_dts.h"
#include "Shell_dts.h"


#if TARGET_CPU_68K
# pragma a6frames on
#endif

#ifdef DEBUG_VERSION
static void NewFailed( void );
#endif

#ifdef DEBUG_VERSION
	#define MALLOCFUNC(x, s) 	NewAlloc( (x), (s) )
	#define	NEWFAILFUNC()		NewFailed()
#else
	#define MALLOCFUNC(x)		NewAlloc(x)
	#define	NEWFAILFUNC()
#endif


/*
**	operator new()	-- regular variant; can throw bad_alloc
*/
/************************************************************************/
/*	Purpose..: 	Allocate memory											*/
/*	Input....:	size of memory to allocate								*/
/*	Return...:	pointer to memory or 0L									*/
/************************************************************************/
void *
operator new( std::size_t size ) THROWS_BAD_ALLOC
{
	void * ptr;
	for(;;)
		{
#ifdef DEBUG_VERSION_TAGS
		if ( ( ptr = MALLOCFUNC( size, nil ) ) != NULL )
			break;
#else
		if ( ( ptr = MALLOCFUNC( size ) ) != NULL )
			break;
#endif
		// get ahold of the current new-handler
		std::new_handler nh = std::set_new_handler( 0 );
		// reinstall it
		std::set_new_handler( nh );
		
		if ( 0 == nh )
			{
			NEWFAILFUNC();
			throw std::bad_alloc();
			break;
			}
		
		nh();
		}
	return ptr;
}


#ifdef DEBUG_VERSION_TAGS
/*
**	operator new() -- debug variant: tagged, non-throwing
*/
/************************************************************************/
/*	Purpose..: 	Allocate memory											*/
/*	Input....:	size of memory to allocate								*/
/*	Return...:	pointer to memory or 0L									*/
/************************************************************************/
void *
operator new( std::size_t size, const NewTag * tagString ) DOES_NOT_THROW
{
	void * ptr;
	for(;;)
		{
		ptr = MALLOCFUNC( size, (const char*) tagString );
		if ( ptr )
			break;
		
		std::new_handler nh = std::set_new_handler( 0 );
		std::set_new_handler( nh );
		if ( 0 == nh )
			{
			NEWFAILFUNC();
			break;
			}
		try
			{
			nh();
			}
		catch ( const std::bad_alloc& )
			{
			break;
			}
		}
	return ptr;
}
#endif	/* DEBUG_VERSION_TAGS */


/*
**	operator new() -- non-throwing variant
*/
/************************************************************************/
/*	Purpose..: 	Allocate memory											*/
/*	Input....:	size of memory to allocate								*/
/*	Return...:	pointer to memory or 0L									*/
/************************************************************************/
void *
operator new( std::size_t size, const std::nothrow_t& ) DOES_NOT_THROW
{
	try
		{
		return operator new( size );
		}
	catch(...) {}
	return 0;
}


/*
**	operator delete() -- non-throwing
**
*/
/************************************************************************/
/*	Purpose..: 	Dispose memory											*/
/*	Input....:	pointer to memory or 0L (no action if 0L)				*/
/*	Return...:	---														*/
/************************************************************************/
void
operator delete( void * ptr ) DOES_NOT_THROW
{
	if ( ptr )
		NewFree( static_cast<char *>( ptr ) );
}


/************************************************************************/
/*	Purpose..: 	Array allocation/deallocation functions					*/
/*	Input....:	---														*/
/*	Return...:	---														*/
/************************************************************************/

#pragma mark -


/*
**	operator new[] -- no tags; can throw exception
*/
extern void *
operator new[](std::size_t size) THROWS_BAD_ALLOC
{
	return operator new(size);
}

#ifdef DEBUG_VERSION
/*
**	operator new[] -- NewTag version, can throw exception
*/
extern void *
operator new[]( std::size_t size, const NewTag * tagString ) THROWS_BAD_ALLOC
{
	return operator new( size, tagString );
}
#endif	/* DEBUG_VERSION */


/*
**	operator new[] -- no tags; does not throw
*/
extern void *
operator new[]( std::size_t size, const std::nothrow_t& nt ) DOES_NOT_THROW
{
	return operator new( size, nt );
}


/*
**	operator delete[]	-- does not throw
*/
extern void
operator delete[]( void * ptr ) DOES_NOT_THROW
{
	operator delete( ptr );
}


#if __MWERKS__ >= 0x2400
/*
**	operator delete() (no_throw variant)
*/
extern __declspec(weak)
void
operator delete( void * ptr, const std::nothrow_t& ) DOES_NOT_THROW
{
    operator delete( ptr );
}


/*
**	operator delete[]() -- (no-throw variant)
*/
extern __declspec(weak)
void
operator delete[]( void* ptr, const std::nothrow_t& ) DOES_NOT_THROW
{
    operator delete[]( ptr );
}
#endif // __MWERKS__


#ifdef DEBUG_VERSION
void
NewFailed( void )
{
#ifdef DTS_Mac
	VDebugStr( "New failed to allocate memory." );
#else
	extern void log( const char * );
	log( "New failed to allocate memory.\n" );
#endif
	extern void DumpBlocks( const char * );
	DumpBlocks( "MemFull.dump" );
}
#endif	/* 	DEBUG_VERSION */


#if TARGET_CPU_68K
# pragma a6frames reset
#endif


#endif	// 0
