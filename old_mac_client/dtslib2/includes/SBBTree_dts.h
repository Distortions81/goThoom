/*
**	SBBTree_dts.h		dtslib2
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

#ifndef SBBTree_dts_h
#define SBBTree_dts_h

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif


/*
**	Self Balancing Binary Trees
*/
// some necessary private interface evil
class _PrivateTree;

typedef int  ( _PrivateTree::*_PrivateCompareFunction )( const _PrivateTree * testNode ) const;
typedef void ( _PrivateTree::*_PrivateIterateFunction )();

_PrivateTree *	SBBTree_Insert(
					_PrivateTree *				rootNode,
					_PrivateTree *				newNode,
					_PrivateCompareFunction		compareFunction );

_PrivateTree *	SBBTree_Find(
					const _PrivateTree *		rootNode,
					const _PrivateTree *		testNode,
					_PrivateCompareFunction		compareFunction );

void			SBBTree_Iterate(
					_PrivateTree *				rootNode,
					_PrivateIterateFunction		fn );

template <class T>
class SBBTree
{
private:
				SBBTree( const SBBTree& );
	SBBTree&	operator=( const SBBTree& );
	
public:
	T *		sbbtLeft;
	T *		sbbtRight;
	int		sbbtCount;
	
	// constructor/destructor
				SBBTree() :
					sbbtLeft(),
					sbbtRight(),
					sbbtCount( 1 ) {}
	
	virtual		~SBBTree() {}
	
	typedef int (T::*CompareFunction)( const T * testNode ) const;
	
	T *			Insert( T * newNode )
		{
		CompareFunction compareFunction = &T::Compare;
		return reinterpret_cast<T *>(
			SBBTree_Insert(
				reinterpret_cast<_PrivateTree *>( this ),
				reinterpret_cast<_PrivateTree *>( newNode ),
			  * reinterpret_cast<_PrivateCompareFunction *>( &compareFunction ) ) );
		}
	
	// find a node in the tree
	T *			Find( const T * testNode ) const
		{
		CompareFunction compareFunction = &T::Compare;
		const _PrivateTree * top = reinterpret_cast<const _PrivateTree *>( this );
		const _PrivateTree * test = reinterpret_cast<const _PrivateTree *>( testNode );
		_PrivateCompareFunction fn = reinterpret_cast<_PrivateCompareFunction>( compareFunction );
		return reinterpret_cast<T *>( SBBTree_Find( top, test, fn ) );
		}
	
	// call the function for every node in the tree in order
	typedef void (T::*IterateFunction)();
	
	void		Iterate( IterateFunction fn )
		{
		SBBTree_Iterate( reinterpret_cast<_PrivateTree *>( this ),
				 * reinterpret_cast<_PrivateIterateFunction *>( &fn ) );
		}
	
	// delete entire tree
	// and example of using the iterate function
	void		DeleteThis()
		{
		delete this;
		}
	
	void		Delete()
		{
		Iterate( &SBBTree<T>::DeleteThis );
		}
};


#endif	// SBBTree_dts_h
