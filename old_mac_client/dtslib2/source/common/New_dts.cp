/*
**	New_dts.cp		dtslib2
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

// Just for fun...
// If you turn DONT_USE_DTS_ALLOC on, you completely bypass the DTSLib memory manager;
// instead you use the C++ library's own unadulterated operator new()/delete() & friends.
// (Historical note: the DTSLib memory manager in Memory_dts.cp was created because,
// at that time, there were serious performance issues in the C++ library as supplied
// with the then-current Metrowerks tools (circa CW Pro3 or Pro4). Specifically, the
// CLServer seemed to have a memory usage pattern that elicited truly lousy performance
// from MSL 3/4. 
// We've kept our version around ever since. But no one has gone back to measure the
// performance of today's allocators (CW Pro 10, g++3.3, etc.), so we have no idea whether
// those libraries have gotten any better.)

// Update: preliminary testing with MacOSX's standard C-library malloc() and friends
// seems to indicate that their performance is indeed better than ours, or at least
// not grossly worse. Haven't had a chance to compare current MSL performance, through.
// So: let's switch over, for the Unix builds anyway.
// (Unless we are tracking allocations via NewTags.)

// One other point: It is not clear whether the DTSLib memory manager is thread-safe,
// whereas malloc/free/operator new et al. are generally almost always so. Thus if there's
// any chance of a DTSLib client operating in a multi-threaded environment, it's pretty much
// mandatory to set this flag.

// Finally: if building with Xcode, then these custom allocators cause untold pain.

#if (defined( DTS_Unix ) || defined( DTS_XCODE ) || TARGET_API_MAC_OSX ) && ! defined( DEBUG_VERSION_TAGS )
# define DONT_USE_DTS_ALLOC		1
#else
# define DONT_USE_DTS_ALLOC		0
#endif

#if ! DONT_USE_DTS_ALLOC

#include "New_dts.h"

/*
This file implements our custom overrides of operator new()
and operator delete().
*/

/*
**	Entry Routines
*/
/*
//	C++ standard-mandated
void *	operator new( size_t size, const std::nothrow_t& ) DOES_NOT_THROW;
void *	operator new( size_t size ) THROWS_BAD_ALLOC;
void *	operator new[]( size_t size ) THROWS_BAD_ALLOC;
void *	operator new[]( size_t size, const std::nothrow_t& nt ) DOES_NOT_THROW;

void	operator delete( void * ptr, const std::nothrow_t& ) DOES_NOT_THROW;
void	operator delete( void * ptr ) DOES_NOT_THROW;
void	operator delete[]( void * ptr ) DOES_NOT_THROW;
void	operator delete[]( void * ptr, const std::nothrow_t& nt ) DOES_NOT_THROW;
void	operator delete( void * ptr, std::size_t ) DOES_NOT_THROW;
void	operator delete[]( void * ptr, std::size_t ) DOES_NOT_THROW;

//	DTSLib debugging additions
void * operator new(   size_t size, const NewTag * tagString ) DOES_NOT_THROW;
void * operator new[]( size_t size, const NewTag * tagString ) DOES_NOT_THROW;
*/


/*
**	Internal Definitions
*/
class NewTag {};


/*
**	Internal Routines
*/
static void *	NewTagFunc( size_t size, const NewTag * tagString ) DOES_NOT_THROW;
static void		DeleteTagFunc( void * ptr ) DOES_NOT_THROW;


/*
**	External routines
**
**	imported from Memory_dts.cp; we do NOT want these
**	exposed in any external header.
*/
#ifdef DEBUG_VERSION_TAGS
void *	NewAlloc( size_t size, const char * tagString ) DOES_NOT_THROW;
#else
void *	NewAlloc( size_t size ) DOES_NOT_THROW;
#endif
void	NewFree( void * ptr ) DOES_NOT_THROW;


/*
**	NewTagFunc()
**
**	called by operator new() to deal with new_handler issues.
*/
#ifndef DEBUG_VERSION_TAGS
void *
NewTagFunc( size_t size, const NewTag * ) DOES_NOT_THROW
#else
void *
NewTagFunc( size_t size, const NewTag * tagString ) DOES_NOT_THROW
#endif
{
	void * ptr;
	for (;;)
		{
#ifndef DEBUG_VERSION_TAGS
		ptr = NewAlloc( size );
#else
		ptr = NewAlloc( size, (const char *) tagString );
#endif
		if ( ptr )
			break;
		std::new_handler nh = std::set_new_handler( nullptr );
		if ( nullptr == nh )
			break;
		std::set_new_handler( nh );
		(*nh)();
		}
	return ptr;
}


/*
**	DeleteTagFunc()
**
**	called by operator delete(). Merely here for symmetry.
*/
void
DeleteTagFunc( void * ptr ) DOES_NOT_THROW
{
	NewFree( static_cast<char *>( ptr ) );
}

#pragma mark -


/*
**	operator new()
**	[throwing permitted (in theory but not in practice)]
*/
void *
operator new( size_t size ) THROWS_BAD_ALLOC
{
	return NewTagFunc( size, nullptr );
}


/*
**	operator new(), in many many flavors
**	[explicitly non-throwing]
*/
void *
operator new( size_t size, const std::nothrow_t& ) DOES_NOT_THROW
{
	return NewTagFunc( size, nullptr );
}


/*
**	operator new()
**	[for arrays; explicitly non-throwing]
*/
void *
operator new[]( size_t size, const std::nothrow_t& /*nt*/ ) DOES_NOT_THROW
{
	return NewTagFunc( size, nullptr );
}


/*
**	operator new()
**	[for arrays; throwing permitted (in theory but not in practice)]
*/
void *
operator new[]( size_t size ) THROWS_BAD_ALLOC
{
	return NewTagFunc( size, nullptr );
}


/*
**	operator delete()
**	[common or garden variety]
*/
void
operator delete( void * ptr ) DOES_NOT_THROW
{
	DeleteTagFunc( ptr );
}


/*
**	operator delete()
**	[explicitly non-throwing]
*/
void
operator delete( void * ptr, const std::nothrow_t& ) DOES_NOT_THROW
{
	DeleteTagFunc( ptr );
}


/*
**	operator delete()
**	[sized variety -- c++11?]
*/
void
operator delete( void * ptr, std::size_t /* sz */ ) DOES_NOT_THROW
{
	DeleteTagFunc( ptr );
}


/*
**	operator delete[]()
**	[for array; no throwspec]
*/
void
operator delete[]( void * ptr ) DOES_NOT_THROW
{
	DeleteTagFunc( ptr );
}


/*
**	operator delete[]()
**	[for arrays; explicitly non-throwing]
*/
void
operator delete[]( void * ptr, const std::nothrow_t& /*nt*/ ) DOES_NOT_THROW
{
	DeleteTagFunc( ptr );
}


/*
**	operator delete[]()
**	[for array; sized version (c++11?)]
*/
void
operator delete[]( void * ptr, std::size_t /* sz */ ) DOES_NOT_THROW
{
	DeleteTagFunc( ptr );
}


#ifdef DEBUG_VERSION_TAGS
/*
**	operator new() -- NewTag version
*/
void *
operator new( size_t size, const NewTag * tagString ) DOES_NOT_THROW
{
	return NewTagFunc( size, tagString );
}


/*
**	operator new[]()	-- NewTag version
*/
void *
operator new[]( size_t size, const NewTag * tagString ) DOES_NOT_THROW
{
	return NewTagFunc( size, tagString );
}
#endif	// DEBUG_VERSION_TAGS

#endif	// ! DONT_USE_DTS_ALLOC

