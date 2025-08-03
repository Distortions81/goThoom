/*
**	TextField_mac.h		dtslib2
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

#ifndef TextField_mac_h
#define TextField_mac_h

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "Local_dts.h"

// are we using MLTE (<MacTextEditor.h>) instead of TextEdit?
// (The MLTE code is unfinished; don't use it)
#define TEXTFIELD_USES_MLTE		0


class DTSTextFieldPriv
{
public:
#if TEXTFIELD_USES_MLTE
	TXNObject	textTEHdl;
#else
	TEHandle	textTEHdl;
	
	int			textLastStart;
	int			textLastStop;
	int			textUndoStart;
	int			textUndoStop;
	const char * textUndoBuff;
	int			textAnchor;
#endif  // ! TEXTFIELD_USES_MLTE
	bool		textActive;
//	bool		textReadOnly;		// if true, can't be typed into (future use, e.g. for drag & drop)
	
	// constructor/destructor
			DTSTextFieldPriv();
			~DTSTextFieldPriv();
	
	// interface
#if ! TEXTFIELD_USES_MLTE
	void	TextToUndo();
#endif
	void	GetSelect( int * start, int * stop ) const;
	int		GetTextLength() const;
	void	GetText( char * text, size_t bufferSize ) const;
	
private:
	// declared but not defined
			DTSTextFieldPriv( const DTSTextFieldPriv& );
	DTSTextFieldPriv&	operator=( const DTSTextFieldPriv& );
};

#endif	// TextField_mac_h

