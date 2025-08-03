/*
**	Cursor_mac.cp		dtslib2
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

#include "Cursor_dts.h"
#include "LinkedList_dts.h"
#include "Memory_dts.h"
#include "Shell_mac.h"


/*
**	DTSShowMouse();
**	DTSHideMouse();
**	DTSObscureMouse();
**	DTSShowArrowCursor();
**	DTWShowWatchCursor();
**	DTSShowCursor();
*/


/*
**	Definitions
*/
class DTSCursor : public DTSLinkedList<DTSCursor>
{
public:
	int				cursorID;
#ifndef AVOID_QUICKDRAW
	CursHandle		cursorHandle;
#endif
	
	// constructor/destructor
						DTSCursor();
						~DTSCursor();

	static DTSCursor *	FetchCursor( int crsrID );
	
private:
				// no copying
						DTSCursor( const DTSCursor& );
	DTSCursor&			operator=( const DTSCursor& );
};


/*
**	Internal Routines
*/


/*
**	Private Library Variables
*/


/*
**	Internal Variables
*/
#ifndef AVOID_QUICKDRAW
static DTSCursor *	gCursorList;
#endif
static DTSCursor *	gLastUsedCursor;



/*
**	DTSShowMouse()
*/
void
DTSShowMouse()
{
#ifndef AVOID_QUICKDRAW
	::ShowCursor();
#endif
}


/*
**	DTSHideMouse()
*/
void
DTSHideMouse()
{
#ifndef AVOID_QUICKDRAW
	::HideCursor();
#endif
}


/*
**	DTSObscureMouse()
*/
void
DTSObscureMouse()
{
#ifndef AVOID_QUICKDRAW
	::ObscureCursor();
#endif
}


/*
**	DTSShowArrowCursor()
*/
void
DTSShowArrowCursor()
{
	::SetThemeCursor( kThemeArrowCursor );
	gLastUsedCursor = nullptr;
}


/*
**	DTSShowWatchCursor()
*/
void
DTSShowWatchCursor()
{
	::SetThemeCursor( kThemeWatchCursor );
	gLastUsedCursor = nullptr;
}


/*
**	DTSShowCursor()
*/
void
DTSShowCursor( int crsrID )
{
#ifndef AVOID_QUICKDRAW
	if ( DTSCursor * cursor = DTSCursor::FetchCursor( crsrID ) )
		{
		gLastUsedCursor = cursor;
		
		if ( cursor->cursorHandle )
			::SetCursor( *cursor->cursorHandle );
		}
#else
# pragma unused( crsrID )
#endif
}


/*
**	DTSCursor::DTSCursor()
*/
DTSCursor::DTSCursor() :
	cursorID( -1 )
#ifndef AVOID_QUICKDRAW
	, cursorHandle( nullptr )
#endif
{
}


/*
**	DTSCursor::~DTSCursor()
*/
DTSCursor::~DTSCursor()
{
	if ( gLastUsedCursor == this )
		DTSShowArrowCursor();
}


/*
**	FetchCursor()		[static]
*/
DTSCursor *
DTSCursor::FetchCursor( int crsrID )
{
#ifdef AVOID_QUICKDRAW
# pragma unused( crsrID )
	return nullptr;
#else
	// first look for the ID in the linked list
	for ( DTSCursor * test = gCursorList;  test;  test = test->linkNext )
		{
		if ( crsrID == test->cursorID )
			return test;
		}
	
	// create the cursor
	DTSCursor * cursor = NEW_TAG("DTSCursor") DTSCursor;
	ReturnPointer( cursor, nullptr );
	
	// attempt to fetch the CURS resource
	CursHandle ch = ::GetCursor( crsrID );
	if ( not ch )
		{
		delete cursor;
		SetErrorCode( resNotFound );
		return nullptr;
		}
	
	// init the cursor
	assert( cursor );
	cursor->linkNext    = gCursorList;
	cursor->cursorID    = crsrID;
//	::HNoPurge( reinterpret_cast<Handle>( ch ) );
	::HLockHi(  reinterpret_cast<Handle>( ch ) );
	cursor->cursorHandle = ch;
	gCursorList         = cursor;
	
	return cursor;
#endif  // ! AVOID_QUICKDRAW
}


