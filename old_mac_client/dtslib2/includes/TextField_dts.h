/*
**	TextField_dts.h		dtslib2
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

#ifndef TextField_dts_h
#define TextField_dts_h

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "Local_dts.h"
#include "Platform_dts.h"
#include "View_dts.h"


/*
**	Define DTSTextField class
*/
enum
{
	kTEAnchorLeft  = -1,
	kTEAnchorNone  = 0,
	kTEAnchorRight = +1
};


struct TFFormat
{
	uint	tfNumLines;
	uint	tfLineStarts[1];	// actually contains tfNumLines entries
};


class DTSTextFieldPriv;

class DTSTextField : public DTSView
{
public:
	DTSImplementNoCopy<DTSTextFieldPriv> priv;
	
	// interface routines
	DTSError	Init( DTSView * parent, const DTSRect * box,
					  const char * fontname = nullptr, int fontsize = 0 );
	void		Activate();
	void		Deactivate();
	void		SetText( const char * text );
	int			GetTextLength() const;
	void		GetText( char * text, size_t bufferSize ) const;
	void		SelectText( int start, int stop );
	void		GetSelect( int * start, int * stop ) const;
	void		Insert( const char * text );
	void		Copy() const;
	void		Paste();
	void		Clear();
	void		Undo();
	void		SetJust( int just );
	TFFormat *	GetFormat() const;	// caller is responsible for disposing of the memory
	int			GetTopLine() const;
	void		SetTopLine( int newTopLine );
	int			GetVisLines() const;
	bool		SetAutoScroll( bool );	// sets autoscroll mode; returns prev style
//	bool		SetReadOnly( bool ro );	// for future use
	
	// overloaded focus routines
	virtual bool	KeyStroke( int key, uint modifiers );
	virtual void	Idle();
	
	// semi-private interface routines
	virtual void	DoDraw();
	virtual bool	Click( const DTSPoint& loc, ulong time, uint modifiers );
	virtual void	Move( DTSCoord left, DTSCoord top );
	virtual void	Size( DTSCoord width, DTSCoord height );
	virtual void	Show();
	virtual void	Hide();
};


#endif	// TextField_dts_h
