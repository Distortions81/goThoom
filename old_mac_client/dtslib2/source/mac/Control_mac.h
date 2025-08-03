/*
**	Control_mac.h		dtslib2
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

#ifndef CONTROL_MAC_H
#define CONTROL_MAC_H

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "Control_dts.h"

// Given a Mac HIViewRef, it may sometimes be necessary to retrieve a pointer to its owning
// DTSControl object.  Here are tags for Get/SetControlProperty() so we can do just that.
enum
{
	kDTSControlOwnerCreator	= 'DTSl',
	kDTSControlOwnerTag		= 'pCtl'
};


/*
**	DTSControlPriv class
*/
class DTSControlPriv
{
public:
	HIViewRef	controlHdl;
//	int			controlType;
		// these fields are obsolete: just use {Get,Set}{Value,Max,Min} accessors
//	int			controlMin;			// min value
//	int			controlMax;			// max value
//	int			controlValue;		// current value
		// and these are pointless
//	int			controlDelay;		// delay before auto changing
//	int			controlSpeed;		// auto changing speed
	
	int			controlDeltaArrow;	// change amount when user clicks in arrow
	int			controlDeltaPage;	// change amount when user clicks in page region
	
	// constructor/destructor
					DTSControlPriv();
					~DTSControlPriv();
	
	// private interface
	DTSError		InitPriv( int ctype, const char * title, WindowRef w, DTSControl * dctl );
	
private:
	// no copying
					DTSControlPriv( const DTSControlPriv& );
	DTSControlPriv&	operator=( const DTSControlPriv& );
};


/*
**	formerly this was an overloaded type-cast
*/
inline HIViewRef
DTS2Mac( const DTSControl * c )
{
	if ( c && c->priv.p )
		return c->priv.p->controlHdl;
	return nullptr;
}

#endif	// CONTROL_MAC_H
