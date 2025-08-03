/*
**	ListView_cl.cp		ClanLord
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

#include "ListView_cl.h"


/*
**	ListView::ListView()
**	constructor
*/
ListView::ListView() :
	lvScrollBar( this ),
	lvCellHeight( 0 ),
	lvStartNum( 0 ),
	lvShowNum( 0 ),
	lvNumNodes( 0 ),
	lvLastClickTime( 0 ),
	lvWindow( nullptr )
{
}


/*
**	ListView::Init()
**
**  place a list in a window and assign properties
*/
void 
ListView::Init(	DTSView * inView, DTSWindow * inParentWindow, int inCellHeight )
{
	Attach( inView );
	
	lvCellHeight = inCellHeight;
	lvWindow = inParentWindow;
	
	// Create the scroll bar
	lvScrollBar.Init( kControlVertScrollBar, "", inParentWindow );
	lvScrollBar.SetDeltaArrow( 1 );
	lvScrollBar.Show();
	
	// install scroll-wheel handler
	if ( noErr == InitTarget( GetWindowEventTarget( DTS2Mac( inParentWindow ) ) ) )
		{
		const EventTypeSpec evt = { kEventClassMouse, kEventMouseWheelMoved };
		AddEventType( evt );
		}
}


/*
**	ListView::Size()
**
**  size the list to an integral number of cells
*/ 
void 
ListView::Size( DTSCoord width, DTSCoord height )
{
	// Round the height to an integral number of cells
	lvShowNum = height / lvCellHeight;
	height = lvShowNum * lvCellHeight;
	
	DTSView::Size( width, height );
	
	// Invalidate any new nodes that are showing
	AdjustRange();
}


/*
**	ListView::Click()
**
**  translate a click into the list into a click to an indexed node
*/
bool
ListView::Click( const DTSPoint& start, ulong time, uint modifiers )
{
	// translate to "list-local" coordinates
	DTSPoint click = start;
	DTSRect bounds;
	GetBounds( &bounds );
	click.Offset( -bounds.rectLeft, -bounds.rectTop );
	
	int inIndex = lvStartNum + click.ptV / lvCellHeight;
	click.ptV %= lvCellHeight;
	
	bool bHandled = ClickNode( inIndex, click, time, modifiers );
	lvLastClickTime = time;
	
	return bHandled;
}


/*
**	ListView::KeyStroke()
**
**  handle keys that change the part of the list showing
*/  
bool
ListView::KeyStroke( int ch, uint /*modifiers*/ )
{
	bool bMustRedraw = false;
	switch ( ch )
		{
		case kPageUpKey:
			{
			int num = lvStartNum - lvShowNum - 1;
			if ( num < 0 )
				num = 0;
			lvStartNum = num;
			bMustRedraw = true;
			} break;
		
		case kPageDownKey:
			{
			int num = lvStartNum + lvShowNum - 1;
			if ( num > lvNumNodes - lvShowNum )
				num = lvNumNodes - lvShowNum;
			if ( num < 0 )
				num = 0;
			lvStartNum = num;
			bMustRedraw = true;
			} break;
		
		case kHomeKey:
			lvStartNum = 0;
			bMustRedraw = true;
			break;
		
		case kEndKey:
			lvStartNum = lvNumNodes - lvShowNum;
			bMustRedraw = true;
			break;
		}
	
	if ( bMustRedraw )
		{
		lvWindow->Invalidate();
		AdjustRange();  // Adjust scroll bars second and it looks quicker...
		}
	
	return bMustRedraw;
}


/*
**  ListView::DoDraw()
**
**  Draws all [visible] nodes of the list
*/
void
ListView::DoDraw()
{
	// get this node's bounding box
	DTSRect r;
	GetBounds( &r );
	r.rectBottom = r.rectTop + lvCellHeight;
	
	bool bActive = IsWindowActive( DTS2Mac( lvWindow ) );
	
	// Refresh any parts of the background that need it
	for ( int i = 0; i < lvShowNum; ++i )
		{
		// Draw the nodes that are there
		// Erase nodes that AREN'T there
		if ( i < lvNumNodes )
			DrawNode( lvStartNum + i, r, bActive );
		else
			Erase( &r );
		
		// next node
		r.Offset( 0, lvCellHeight );
		}
}


/*
**	ListView::Update()
*/
void
ListView::Update() const
{
	if ( lvWindow )
		lvWindow->Invalidate();
}


/*
**  ListView::ShowNode()
**
**  Scrolls a node into view
**  Doesn't update the view
*/
void 
ListView::ShowNode( int inIndex )
{
	if ( inIndex < lvStartNum
	||	 inIndex >= lvStartNum + lvShowNum )
		{
		int val = inIndex - ( lvShowNum / 2 );
		lvScrollBar.SetValue( val );
		lvStartNum = lvScrollBar.GetValue();
		}
}


/*
**	ListView::AdjustRange()
**
**  adjusts the range of the scrollbar and then redraws it
**  should be called after a change in the number of nodes
**	(or when the window is resized)
*/ 
void
ListView::AdjustRange()
{
	int max = lvNumNodes - lvShowNum;	
	
	// Adjust the actual max for the scroll bar
	if ( max < 0 )
		max = 0;
	lvScrollBar.SetRange( 0, max );
	lvScrollBar.SetDeltaPage( lvShowNum - 1 );
	lvScrollBar.SetThumbSize( lvShowNum );
	
	// Do the test for the max first so that min has priority.
	// This makes sure that things behave properly when the list is small.
	if ( lvStartNum > max )
		lvStartNum = max;
	if ( lvStartNum < 0 )
		lvStartNum = 0;
	
	lvScrollBar.SetValue( lvStartNum );
}


// scroll wheel handling

/*
**	ListView::HandleEvent()
*/
OSStatus
ListView::HandleEvent( CarbonEvent& evt )
{
	OSStatus result = eventNotHandledErr;
	
	OSStatus err;
	
	// handle mouse-wheels
	if ( kEventClassMouse == evt.Class()
	&&	 kEventMouseWheelMoved == evt.Kind() )
		{
		EventMouseWheelAxis axis;
		SInt32 delta;
		
		err = evt.GetParameter( kEventParamMouseWheelAxis,
					typeMouseWheelAxis, sizeof axis, &axis );
		if ( noErr == err )
			err = evt.GetParameter( kEventParamMouseWheelDelta,
					typeSInt32, sizeof delta, &delta );
		
		if ( noErr == err )
			{
			if ( kEventMouseWheelAxisY == axis )
				{
				int oldVal = lvScrollBar.GetValue();
				lvScrollBar.SetValue( oldVal - delta );
				
				// SetValue() will clamp to current limits. If the value has still
				// changed, even after that, then we should scroll to the new position
				if ( oldVal != lvScrollBar.GetValue() )
					lvScrollBar.Hit();
				}
			result = noErr;
			}
		}
	return result;
}

#pragma mark -


/*
**	ListViewScrollBar::Hit()
**
**	handle a click in the vertical scroll bar
*/
void
ListViewScrollBar::Hit()
{
	int value = GetValue();
	if ( lvsbListView->lvStartNum != value )
		{
		lvsbListView->lvStartNum = value;
		lvsbListView->Draw();
		}
}


