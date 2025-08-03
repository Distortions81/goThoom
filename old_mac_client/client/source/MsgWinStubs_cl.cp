/*
**	MessageWin.sh.cpp		ClanLord, CLServer
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

#ifndef DEBUG_VERSION

/*
**	Entry Routines
*/
/*
void	CreateMsgWin( int numLines, int kind, const DTSRect * box );
void	CloseMsgWin();
void	GetMsgWinPos( DTSPoint * pt );
void	ShowMessage( const char * format, ... );
void	ShowDump( const void * dataPtr, size_t count );
void	SaveMsgLog( const char * fname );
*/


/*
**	CreateMsgWin()
*/
void
CreateMsgWin( int /* numLines */, int /* kind */, const DTSRect * /* box */ )
{
}


/*
**	CloseMsgWin()
**	close the message window and the file
*/
void
CloseMsgWin()
{
}


/*
**	ShowMessage()
**	show the message
*/
void
ShowMessage( const char * /* format */, ... )
{
}


/*
**	ShowDump()
**	show a hex dump of the memory location
*/
void
ShowDump( const void * /* dataPtr */, size_t /* count */ )
{
}


/*
**	GetMsgWinPos()
**	return the position of the message window
*/
void
GetMsgWinPos( DTSPoint * /* pos */, DTSPoint * /* size */ /* =0 */ )
{
}


/*
**	SaveMsgLog()
**	set the flag the decides if we save to the message log or not
*/
void
SaveMsgLog( const char * /* fname */ )
{
}

#endif  // ndef DEBUG_VERSION
