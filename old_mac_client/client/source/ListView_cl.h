/*
**	ListView_cl.h		ClanLord
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

#ifndef LISTVIEW_CL_H
#define LISTVIEW_CL_H

// class ListView
// class ListViewScrollBar

// A view with a slave scroll bar
// Handles the display part of a list but not the storage part

class ListView;

class ListViewScrollBar : public DTSControl
{
	ListView * lvsbListView;

public:
					ListViewScrollBar( ListView * inLV ) : lvsbListView( inLV ) {}
	void			Hit();
};


class ListView : public DTSView, public CarbonEventResponder
{
public:
	ListViewScrollBar	lvScrollBar;
	
	int			lvCellHeight;		// height of a cell in pixels
	
	// Nodes are numbered starting at zero
	int			lvStartNum;		// first showing node
	int			lvShowNum;		// number of nodes showing
	int			lvNumNodes;		// total number of nodes
	
protected:
	ulong		lvLastClickTime;
	
	DTSWindow * lvWindow;

public: 
			ListView();
	
	void	Init( DTSView * inView, DTSWindow * inParentWindow, int inCellHeight );
		// Place the view in a new window

	virtual void Size( DTSCoord width, DTSCoord height );

	virtual void DoDraw();
	virtual bool KeyStroke( int ch, uint modifiers );
	virtual bool Click( const DTSPoint& start, ulong time, uint modifiers );
	
	void	ShowNode( int inIndex );			// scroll a node into view, doesn't redraw 
	void	AdjustRange();						// the number of nodes changed
	void	Update() const;						// tell owner window to refresh
	
	// Pure virtuals
	virtual void DrawNode( int inIndex, const DTSRect& inRect, bool bActive ) = 0;
	virtual bool ClickNode( int inIndex, const DTSPoint& start, ulong time, uint modifiers ) = 0;
	
	virtual OSStatus HandleEvent( CarbonEvent& );
};

#endif  // LISTVIEW_CL_H

