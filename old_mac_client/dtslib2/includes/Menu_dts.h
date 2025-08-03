/*
**	Menus_dts.h		dtslib2
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

#ifndef Menus_dts_h
#define Menus_dts_h

#ifndef _dtslib2_
#include "Prefix_dts.h"
#endif

#include "Local_dts.h"
#include "Platform_dts.h"


/*
**	Define special menu commands sent when the corresponding core apple event is received
*/
enum
{
	mOSNew		= -1,
	mOSOpen		= -2,
	mOSPrint	= -3,
	mOSQuit		= -4
};


/*
**	DTSMenu class
*/
class DTSMenuPriv;
class DTSMenu
{
public:
	DTSImplementNoCopy<DTSMenuPriv> priv;
	
	// constructor/destructor
				DTSMenu();
	virtual		~DTSMenu();
	
	// interface
	DTSError	Init( int menuBarID );
	void		SetFlags( int menuItem, uint flags );
	void		CheckRange( int minItem, int maxItem, int checkItem );
	void		SetText( int menuItem, const char * text );
	void		GetText( int menuItem, char * text, size_t maxlen = 256 ) const;
	void		Delete( int menuItem );
	void		AddMenu( int menuID, int beforeID );	// for hierarchical menus - Jeff
	
	// set auto repeat menu keys flag
	void		SetAutoRepeat( bool flag );
	
	// customizing routines
	virtual bool	DoCmd( long menuItem );		// return true if command handled
	
	// accessors
	bool		GetAutoRepeat() const;
#if defined( __MACHELP__ )
	ushort		GetHelpMenuOffset() const;
#endif
	
	// no copying
private:
				DTSMenu( const DTSMenu& );
	DTSMenu&	operator=( const DTSMenu& );
};

enum
{
	kMenuNormal		= 0,		// enabled and not checked
	kMenuDisabled	= 1,
	kMenuChecked	= 2
};


/*
**	global instance of the menu handler
*/
extern DTSMenu *		gDTSMenu;


#endif  // Menus_dts_h
