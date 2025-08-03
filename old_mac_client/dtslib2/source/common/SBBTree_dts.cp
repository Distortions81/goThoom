/*
**	SBBTree_dts.cp		dtslib2
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
#include "SBBTree_dts.h"


/*
	Self Balancing Binary Tree
	
	a binary tree has a left sub-tree and a right sub-tree.
	they are much faster to search than a linked list.
	they do consume more memory.
	if at all possible one should binary search an array.
	faster still and consumes less memory.
	
	Searching
	
		compare the test node to the root node.
		if equal then we're done.
		if less than then search the left sub-tree.
		otherwise search the right sub-tree.
		searching works best when each test eliminates half of the 
		remaining possibilities.
	
	Iterating
	
		iterating calls a given function for every node in the tree in order.
		use globals to pass data into and out of the function.
		iterate the function on the left sub-tree.
		call the function for the root node.
		iterate the function on the right sub-tree.
		it is safe to delete the tree or insert it into a different tree.
	
	Inserting
	
		search for the appropriate place for the new node.
		if adding the node to the sub-tree will cause the root tree to be 
		out of balance then adjust the tree to one that is more balanced.
	
	Balancing
	
		it is desirable for every node in the tree to have sub trees of
		approximately the same size.
		consider the following three element trees:
		
			A)   O          B)   O          C)   O
			    / \			    /                 \
			   O   O           O                   O
			                  /				        \
			                 O                       O
		
		clearly A is balanced.
		it turns out that by our definition B and C are also balanced.
		the reason is that balancing B and C to A is rather difficult.
		we define the following conditions for balanced trees:
			left  <= 2 * right + 2
			right <= 2 * left  + 2
		this definition keeps us within 1 or 2 of the optimal number
		of comparisons required to search for a node.
		
		in order to balance the tree we do the following transformation:
		
			1)    B                     2)    D
			     / \                         / \
			    A   D       <--->           B   E
			       / \                     / \
			      C   E                   A   C
		where in 1, D > A and in 2, B > E.
		ie 1 is biased to the right and 2 is biased to the left.
		let's suppose we wanted to insert a new node N into 1.
		let's suppose that N comes after B.
		let's suppose that adding N to D will put the tree out of balance.
		if things were easy we could use the transformation to change our
		tree from one that is right biased to one that is left biased and
		then successfully insert N into 2 and be left with a balanced tree.
		that's close but doesn't quite work.
		the failure case is when N also comes before D.
		for some trees it is possible that after the transformation N will
		still be inserted into the larger subtree and unbalance the whole tree.
		fortunately the solution is straightforward.
		we need to take the additional step of guaranteeing that after the
		transformation the tree will still be in balance wherever N goes.
		this can be accomplished in our example by ensuring the D subtree
		is biased to the right ie C < E.
		the proof that it works is left to the reader. ;->
*/


/*
**	Entry Routines
*/
/*
_PrivateTree *	SBBTree_Insert( _PrivateTree *, _PrivateTree *, _PrivateCompareFunction );
_PrivateTree *	SBBTree_Find( _PrivateTree *, _PrivateTree *, _PrivateCompareFunction );
void			SBBTree_Iterate( _PrivateTree *, _PrivateIterateFunction );
*/


/*
**	Definitions
*/
class _PrivateTree : public SBBTree<_PrivateTree>
{
};


/*
**	Internal Routines
*/
static _PrivateTree *	SBBTree_BiasLeft( _PrivateTree * rootNode );
static _PrivateTree *	SBBTree_BiasRight( _PrivateTree * rootNode );


/*
**	Variables
*/


/*
**	SBBTree_Insert()
**
**	generic tree insertion routine
**	yeah, i know i use a couple of evil goto's.
**	the alternatives were unappealing.
*/
_PrivateTree *
SBBTree_Insert( _PrivateTree * rootNode, _PrivateTree * newNode,
				_PrivateCompareFunction compareFunction )
{
	// paranoid check
	if ( not newNode )
		return rootNode;
	
	// the tree is currently empty
	if ( not rootNode )
		{
		newNode->sbbtLeft  =
		newNode->sbbtRight = nullptr;
		newNode->sbbtCount = 1;
		return newNode;
		}
	
	// locals
	int leftCount  = rootNode->sbbtLeft  ? rootNode->sbbtLeft->sbbtCount  : 0;
	int rightCount = rootNode->sbbtRight ? rootNode->sbbtRight->sbbtCount : 0;
	
	// do we go in the left tree or the right?
	int cmp = (rootNode->*compareFunction)( newNode );
	if ( cmp <= 0 )
		{
		// insertion will not unbalance the tree
		if ( leftCount <= rightCount + rightCount + 1 )
			goto insertLeft;
		
		// insertion will unbalance the tree
		// bias the left tree to the left
		// bias the root to the right
		rootNode->sbbtLeft = SBBTree_BiasLeft( rootNode->sbbtLeft );
		rootNode           = SBBTree_BiasRight( rootNode );
		}
	else
		{
		// insertion will not unbalance the tree
		if ( rightCount <= leftCount + leftCount + 1 )
			goto insertRight;
		
		// insertion will unbalance the tree
		// bias the right tree to the right
		// bias the root to the left
		rootNode->sbbtRight = SBBTree_BiasRight( rootNode->sbbtRight );
		rootNode            = SBBTree_BiasLeft( rootNode );
		}
	
	// we changed the tree so it is no longer unbalanced
	// can now just insert the new node into the tree with the new root
	cmp = (rootNode->*compareFunction)( newNode );
	if ( cmp <= 0 )
		{
insertLeft:
		rootNode->sbbtLeft = SBBTree_Insert( rootNode->sbbtLeft, newNode, compareFunction );
		}
	else
		{
insertRight:
		rootNode->sbbtRight = SBBTree_Insert( rootNode->sbbtRight, newNode, compareFunction );
		}
	++rootNode->sbbtCount;
	
	return rootNode;
}


/*
**	SBBTree_Find()
**
**	generic tree find routine
**	nice syntax huh?
*/
_PrivateTree *
SBBTree_Find( const _PrivateTree * rootNode, const _PrivateTree * testNode,
			  _PrivateCompareFunction compareFunction )
{
	_PrivateTree * root = const_cast<_PrivateTree *>( rootNode );
	for(;;)
		{
		if ( not root )
			return nullptr;
		int cmp = (root->*compareFunction)( testNode );
		if ( 0 == cmp )
			return root;
		if ( 0 > cmp )
			root = root->sbbtLeft;
		else
			root = root->sbbtRight;
		}
}


/*
**	SBBTree_Iterate()
**
**	iterate through the whole tree
*/
void
SBBTree_Iterate( _PrivateTree * rootNode, _PrivateIterateFunction fn )
{
	if ( not rootNode )
		return;
	
	// the iterate function might delete the root node!
	// we better grab the sub trees before that happens
	
	_PrivateTree * leftTree  = rootNode->sbbtLeft;
	_PrivateTree * rightTree = rootNode->sbbtRight;
	
	leftTree->Iterate( fn );
	(rootNode->*fn)();
	rightTree->Iterate( fn );
}


/*
**	SBBTree_BiasLeft()
**
**	bias the tree so the left sub-tree is bigger
*/
_PrivateTree *
SBBTree_BiasLeft( _PrivateTree * tt )
{
	_PrivateTree * lt = tt->sbbtLeft;
	_PrivateTree * rt = tt->sbbtRight;
	
	// no right tree
	// already biased to the left
	if ( not rt )
		return tt;
	
	// left tree is already bigger
	int lc = lt ? lt->sbbtCount : 0;
	int rc = rt->sbbtCount;
	if ( lc >= rc )
		return tt;
	
	// get the left tree of the right tree
	// get its size
	// calc the new count for the old root node
	// which is the new right tree
	_PrivateTree * rlt = rt->sbbtLeft;
	int rlc = rlt ? rlt->sbbtCount : 0;
	int tc = tt->sbbtCount - rc + rlc;
	
	// swap the root and the right
	// update their counts
	tt->sbbtRight = rlt;
	rt->sbbtLeft  = tt;
	tt->sbbtCount = tc;
	rt->sbbtCount = rc - rlc + tc;
	
	return rt;
}


/*
**	SBBTree_BiasRight()
**
**	bias the tree so the right sub-tree is bigger
*/
_PrivateTree *
SBBTree_BiasRight( _PrivateTree * tt )
{
	_PrivateTree * lt = tt->sbbtLeft;
	_PrivateTree * rt = tt->sbbtRight;
	
	// no left tree
	// already biased to the right
	if ( not lt )
		return tt;
	
	// right tree is already bigger
	int lc = lt->sbbtCount;
	int rc = rt ? rt->sbbtCount : 0;
	if ( rc >= lc )
		return tt;
	
	// get the right tree of the left tree
	// get its size
	// calc the new count for the old root node
	// which is the new left tree
	_PrivateTree * lrt = lt->sbbtRight;
	int lrc = lrt ? lrt->sbbtCount : 0;
	int tc = tt->sbbtCount - lc + lrc;
	
	// swap the root and the left
	// update their counts
	tt->sbbtLeft  = lrt;
	lt->sbbtRight = tt;
	tt->sbbtCount = tc;
	lt->sbbtCount = lc - lrc + tc;
	
	return lt;
}


