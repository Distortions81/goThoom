/*
**	New_dts.h		dtslib2
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

#ifndef New_dts_h
#define New_dts_h

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include <new>
#include "Memory_dts.h"


#if 0
/*
	This file, New_dts.h, is externally visible.
	It declares all the operator new() and delete() overrides.
	
	Its counterpart, New_dts.cp, implements those using the private
	functions NewTagFunc() and DeleteTagFunc().
	
	Those, in turn, are implemented in Memory_dts.cp.
	
*********************************************************************
instead of using plain ordinary "new", we use "NEW_TAG".
it's in all caps to indicate it's a macro.
for debug builds, NEW_TAG adds a tag string to the allocated memory block.
for non-debug builds, NEW_TAG is the standard new.
well, maybe the new that doesn't throw any exceptions.
for example:

void foo()
{
	char * p = NEW_TAG("FooBuffer") char[100];
	strcpy( p, "Hello, World!" );
	delete[] p;
}

*********************************************************************/

/*
NOTE:
    C++ defines/requires the following 8 functions:
 		[But the rules have changed in C++11; see Local_dts.h]
1    - operator new(size_t) throw(bad_alloc);				// "classic" allocator; can throw
2    - operator new(size_t, const nothrow_t&) throw();		// non-throwing allocator
3    - operator delete(void *) throw();						// "classic" disposer
4    - operator delete(void *, const nothrow_t&) throw();	// disposer called by runtime if storage
															// was originally alloc'ed by #2
															
5    - operator new[](size_t) throw(bad_alloc);				// "classic" array allocator; can throw
6    - operator new[](size_t, const nothrow_t&) throw();	// non-throwing array allocator
7    - operator delete[](void *) throw();					// normal array disposer
8    - operator delete[](void *, const nothrow_t&) throw();	// 8 is to 6 as 4 is to 2

It is necessary to make sure that ALL EIGHT are vectored through the same
underlying allocator. _Most_ of our allocations are wrapped in NEW_TAG,
but we don't have a corresponding DELETE macro; nor can we make any guarantees
about how the C++ runtime handles its own memory needs. That's why we have to
either cover -all- the bases, or none. Trying to mix & match will always eventually
crash somewhere. ISO says that the "default behavior" for #4 is to call #1, and for
#6 to call #2, but I'm not sure we can rely on that.

    When we have DEBUG_VERSION_TAGS on, we also provide these new functions:
    - operator new(size_t, const NewTag *) throw()
    - operator new[](size_t, const NewTag *) throw()

Therefore: First off, the deletes are easy; they merely vector through our NewFree().
The new()'s are trickier, alas. When tagging is OFF, we require this:

    - all four new()'s boil down to NewAlloc(size_t)
    - NEW_TAG() be defined as new(nothrow)

With tagging turned ON, we need to have:
    - all four C++ new()'s boil down to NewAlloc(size_t, (NewTag *) 0 );
    - NEW_TAG() must map to operator new(size_t, const NewTag *)
*/
#endif  // 0


/*
**	the standard C++ new/delete functions.
**	They're declared in <new>.
**	Our custom versions are implemented in New_dts.cp
*/
// void *	operator new( size_t size ) THROWS_BAD_ALLOC;
// void *	operator new( size_t size, const std::nothrow_t& ) DOES_NOT_THROW;
// void *	operator new[]( size_t size ) THROWS_BAD_ALLOC;
// void *	operator new[]( size_t size, const std::nothrow_t& nt ) DOES_NOT_THROW;

// void	operator delete( void * ptr, const std::nothrow_t& ) DOES_NOT_THROW;
// void	operator delete( void * ptr ) DOES_NOT_THROW;
// void	operator delete[]( void * ptr ) DOES_NOT_THROW;
// void	operator delete[]( void * ptr, const std::nothrow_t& nt ) DOES_NOT_THROW;
//	[new in C++11 or thereabouts?]
// void operator delete( void * ptr, std::size_t ) DOES_NOT_THROW;
// void operator delete[]( void * ptr, std::size_t ) DOES_NOT_THROW;


// Now define the NEW_TAG macro appropriately:
//		when DEBUG_VERSION_TAGS is off, NEW_TAG just means new(nothrow)
//		otherwise it invokes our debugging flavor, new( void *, const NewTag * )


#ifndef DEBUG_VERSION_TAGS

// standard old-fashioned non-throwing new
# define NEW_TAG(s) new( std::nothrow )

#else  // DEBUG_VERSION_TAGS

class NewTag;

// fancy-dancy newfangled new that stores a pointer to the tag string
// in the allocated block.
# define NEW_TAG(s) new( (const NewTag *) (s) )

void *	operator new(   size_t size, const NewTag * tagString ) DOES_NOT_THROW;
void *	operator new[]( size_t size, const NewTag * tagString ) DOES_NOT_THROW;

#endif  // DEBUG_VERSION_TAGS

#endif	// New_dts_h
