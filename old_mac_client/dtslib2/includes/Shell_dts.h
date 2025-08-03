/*
**	Shell_dts.h		dtslib2
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

#ifndef Shell_dts_h
#define Shell_dts_h

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "Local_dts.h"
#include "Platform_dts.h"
#include "View_dts.h"


/*
**	DTSApp class
*/
class DTSAppPriv;
class DTSApp : public DTSFocus
{
public:
	DTSImplementNoCopy<DTSAppPriv> priv;
	
	// constructor/destructor
					DTSApp();
	virtual			~DTSApp();
	
	// customizing routines
	virtual void	Suspend();
	virtual void	Resume();
	
	long			SetSleepTime( long newTime );
	
	// overloaded DTSFocus routines
	virtual bool	KeyStroke( int ch, uint modifiers );
	
	// accessors
	uint32_t		GetSignature() const;
	int				GetResFile() const;
	DTSWindow *		GetFrontWindow() const;
};


/*
**	Useful Globals
*/
extern DTSApp *    gDTSApp;			// global instance of the application
extern DTSFileSpec gDTSOpenFile;	// global data file to open


/*
**	Interface routines
*/
void	RunApp();					// run the application


#endif  // Shell_dts_h
