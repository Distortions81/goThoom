/*
**	LinkedList_dts.h		dtslib2
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
*/

#ifndef LinkedList_dts_h
#define LinkedList_dts_h

#ifndef _dtslib2_
#include "Prefix_dts.h"
#endif


/*
**	A basic linked list
*/
class _DTSLinkedList;
void	DTSLinkedList_InstallLast( _DTSLinkedList * elem, _DTSLinkedList *& root );
void	DTSLinkedList_Remove( _DTSLinkedList * elem, _DTSLinkedList *& root );
void	DTSLinkedList_MoveForward( _DTSLinkedList * elem, _DTSLinkedList *& root );
void	DTSLinkedList_MoveBack( _DTSLinkedList * elem, _DTSLinkedList *& root );
uint	DTSLinkedList_Count( const _DTSLinkedList * elem );
void	DTSLinkedList_AppendList( _DTSLinkedList * elem, _DTSLinkedList *& root );
void	DTSLinkedList_DeleteLinkedList( _DTSLinkedList *& root );
void	DTSLinkedList_ReverseLinkedList( _DTSLinkedList *& root );

template< class T >
class DTSLinkedList
{
private:
	// no copying
	DTSLinkedList&		operator=( const DTSLinkedList& );
						DTSLinkedList( const DTSLinkedList& );
	
public:
	T *		linkNext;
	
	// constructor/destructor
					DTSLinkedList() : linkNext( nullptr ) {}
	virtual			~DTSLinkedList() {}
	
	// interface routines
	// install at the front of the linked list
	void	InstallFirst( T *& root )
				{
				linkNext = root;
				root = (T *) this;
				}
	
	// install in the linked list after the one specified
	void	InstallBehind( T * behind )
				{
				linkNext = behind->linkNext;
				behind->linkNext = (T *) this;
				}
	
	// install at the end of the linked list
	void	InstallLast( T *& root )
				{
				DTSLinkedList_InstallLast( (_DTSLinkedList *) this, (_DTSLinkedList *&) root );
				}
	
	// remove from the list
	void	Remove( T *& root )
				{
				DTSLinkedList_Remove( (_DTSLinkedList *) this, (_DTSLinkedList *&) root );
				}
	
	// move an element to the front of the list
	void	GoToFront( T *& root )
				{
				Remove( root );
				InstallFirst( root );
				}
	
	// move an element to the end of the list
	void	GoToBack( T *& root )
				{
				Remove( root );
				InstallLast( root );
				}
	
	// move an element behind another
	void	GoBehind( T *& root, T * behind )
				{
				Remove( root );
				InstallBehind( behind );
				}
	
	// move forward one element
	void	MoveForward( T *& root )
				{
				DTSLinkedList_MoveForward( (_DTSLinkedList *) this, (_DTSLinkedList *&) root );
				}
	
	// move back    one element 
	void	MoveBack( T *& root )
				{
				DTSLinkedList_MoveBack( (_DTSLinkedList *) this, (_DTSLinkedList *&) root );
				}
	
	// number of elements in the list
	uint	Count() const
				{
				return DTSLinkedList_Count( (const _DTSLinkedList *) this );
				}
	
	// append this list to another
	void	AppendList( T *& root )
				{
				DTSLinkedList_AppendList( (_DTSLinkedList *) this, (_DTSLinkedList *&) root );
				}
	
	// delete an entire linked list
	static void DeleteLinkedList( T *& root )
				{
				DTSLinkedList_DeleteLinkedList( (_DTSLinkedList *&) root );
				}
	
	// reverse the order of a linked list
	static void ReverseLinkedList( T *& root )
				{
				DTSLinkedList_ReverseLinkedList( (_DTSLinkedList *&) root );
				}
};


#endif  // LinkedList_dts_h
