/*
**	StyledTextField_cl.h		ClanLord
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

#ifndef StyledTextField_cl_h
#define	StyledTextField_cl_h

#pragma pack( push, 2 )
struct CLStyleRecord
{
	int16_t		font;		// FOND ID
	int16_t		face;		// bold, italic, etc.
	int16_t		size;		// text size
	DTSColor	color;		// color
};
#pragma pack( pop )


class StyledTextField : public DTSTextField
{
public:
	// interface routines
	DTSError	Init( DTSView * parent, DTSRect * box,
					const char * fontname = nullptr, int fontsize = 0 );
	void		StyleInsert( const char * text, const CLStyleRecord * style );
	
	int			GetTopLine() const;
	void		SetTopLine( int newTopLine );
	int			GetNumLines() const;
	int			GetVisLines() const;
	
		// fix the damn scrolling bug once and for all
	bool		AtBottom() const;
	void		ScrollToBottom();
};

#endif	// StyledTextField_cl_h

