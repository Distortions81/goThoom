/*
**	LinkedList_dts.cp		dtslib2
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

#include "Local_dts.h"
#include "LinkedList_dts.h"


/*
**	Entry Routines
*/
/*
void	DTSLinkedList_InstallLast( _DTSLinkedList * elem, _DTSLinkedList *& root );
void	DTSLinkedList_Remove( _DTSLinkedList * elem, _DTSLinkedList *& root );
void	DTSLinkedList_MoveForward( _DTSLinkedList * elem, _DTSLinkedList *& root );
void	DTSLinkedList_MoveBack( _DTSLinkedList * elem, _DTSLinkedList *& root );
uint	DTSLinkedList_Count( const _DTSLinkedList * elem );
void	DTSLinkedList_AppendList( _DTSLinkedList * elem, _DTSLinkedList *& root );
void	DTSLinkedList_DeleteLinkedList( _DTSLinkedList *& root );
void	DTSLinkedList_ReverseLinkedList( _DTSLinkedList *& root );
*/

/*
**	Definitions
*/
class _DTSLinkedList : public DTSLinkedList<_DTSLinkedList>
{
};


/*
**	Internal Routines
*/


/*
**	Internal Variables
*/


/*
**	DTSLinkedList_InstallLast()
**
**	add an element to the end of the linked list
*/
void
DTSLinkedList_InstallLast( _DTSLinkedList * elem, _DTSLinkedList *& root )
{
	// scan to the end of the list
	_DTSLinkedList ** linkp = &root;
	for ( _DTSLinkedList * test; (test = *linkp) != nullptr;  linkp = &test->linkNext )
		;
	
	// install at the end
	elem->linkNext = nullptr;
	*linkp = elem;
}


/*
**	DTSLinkedList_Remove()
**
**	remove an element from the linked list
*/
void
DTSLinkedList_Remove( _DTSLinkedList * elem, _DTSLinkedList *& root )
{
	// scan through the list looking for this element
	_DTSLinkedList ** linkp = &root;
	for ( _DTSLinkedList * test; (test = *linkp) != nullptr;  linkp = &test->linkNext )
		{
		// stop when we find this element
		if ( test == elem )
			{
			// remove it from the list
			*linkp = elem->linkNext;
			break;
			}
		}
	
	// just to be safe
	elem->linkNext = nullptr;
}


/*
**	DTSLinkedList_MoveForward()
**
**	move one element forward in the list
*/
void
DTSLinkedList_MoveForward( _DTSLinkedList * elem, _DTSLinkedList *& root )
{
	// bail if this is the first item in the list
	_DTSLinkedList * test = root;
	if ( test == elem )
		return;
	
	// scan through the list looking for this element
	_DTSLinkedList ** linkp = &root;
	for(;;)
		{
		// peek at the next element
		_DTSLinkedList * next = test->linkNext;
		
		// odd, this element didn't seem to be in the list
		if ( not next )
			break;
		
		// stop when we find this element
		if ( next == elem )
			{
			// rearrange the list
			// old order: link test next tail
			// new order: link next test tail
			test->linkNext = next->linkNext;
			next->linkNext = test;
			*linkp = next;
			break;
			}
		
		// iterate
		linkp = &test->linkNext;
		test = next;
		}
}


/*
**	DTSLinkedList_MoveBack()
**
**	move one element back in the list
*/
void
DTSLinkedList_MoveBack( _DTSLinkedList * elem, _DTSLinkedList *& root )
{
	// bail if this is the last element in the list
	_DTSLinkedList * next = elem->linkNext;
	if ( not next )
		return;
	
	// scan through the list looking for this element
	_DTSLinkedList ** linkp = &root;
	for ( _DTSLinkedList * test; (test = *linkp) != nullptr;  linkp = &test->linkNext )
		{
		// stop when we find this element
		if ( test == elem )
			{
			// rearrange the list
			// old order: root test next tail
			// new order: root next test tail
			elem->linkNext = next->linkNext;
			next->linkNext = elem;
			*linkp = next;
			break;
			}
		}
}


/*
**	DTSLinkedList_Count()
**
**	return the number of elements in the list
*/
uint
DTSLinkedList_Count( const _DTSLinkedList * elem )
{
	uint count = 0;
	for ( const _DTSLinkedList * step = elem;  step;  step = step->linkNext )
		++count;
	
	return count;
}


/*
**	DTSLinkedList_AppendList()
**
**	append this list to another
*/
void
DTSLinkedList_AppendList( _DTSLinkedList * elem, _DTSLinkedList *& root )
{
	// trivial case when the list to be appended to is empty
	_DTSLinkedList * head = root;
	if ( not head )
		{
		root = elem;
		return;
		}
	
	// find the last element in the list
	for(;;)
		{
		_DTSLinkedList * next = head->linkNext;
		if ( not next )
			break;
		head = next;
		}
	
	// append this list in its entirety
	head->linkNext = elem;
}


/*
**	DTSLinkedList_DeleteLinkedList()
**
**	delete every element of a linked list
*/
void
DTSLinkedList_DeleteLinkedList( _DTSLinkedList *& root )
{
	// walk through the list
	_DTSLinkedList * next;
	for ( _DTSLinkedList * test = root;  test;  test = next )
		{
		// avoid using a field from a block of memory
		// that was just freed
		next = test->linkNext;
		delete test;
		}
	
	// just to be safe
	root = nullptr;
}


/*
**	DTSLinkedList_ReverseLinkedList()
**
**	reverse the order of a linked list
*/
void
DTSLinkedList_ReverseLinkedList( _DTSLinkedList *& root )
{
	_DTSLinkedList * elem = root;
	root = nullptr;
	for ( _DTSLinkedList * next;  elem;  elem = next )
		{
		next = elem->linkNext;
		elem->InstallFirst( root );
		}
}


