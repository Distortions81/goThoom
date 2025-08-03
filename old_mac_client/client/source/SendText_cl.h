/*
**	SendText_cl.h		ClanLord
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

#ifndef SENDTEXT_CL_H
#define SENDTEXT_CL_H

class SendTextField : public DTSTextField, public CarbonEventResponder
{
public:
	bool	lastDrawnMacroExecuting;
	
	virtual void	Init( DTSView * parent, const DTSRect * box, const char * font, int sz );
	virtual void	DoDraw();
	void			UpdateMacroStatus( bool macroExecuting );
	
	OSStatus		DoServiceEvent( const CarbonEvent& );
	virtual OSStatus HandleEvent( CarbonEvent& );
};


void	InitSendText( DTSWindow * window, const DTSRect * box );
bool	SendTextKeyStroke( int ch, uint modifiers );
void	GetSendString( char * dst, size_t bufSize );
void	ClearSendString( uint ackCmd );
bool	SendTextCmd( long menuCmd );
bool	SendCommand( const char * cmdName, const char * cmdParam, bool bQuiet );
bool	SendText( const char * inTextToSend, bool bQuiet );
void	InsertName( const char * name );

extern	SendTextField	gSendText;


#endif	// SENDTEXT_CL_H
