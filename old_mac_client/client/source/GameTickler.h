/*
 *  GameTickler.h
 *  ClanLordX
 *
 *  Created by me on 10/13/16.
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

#ifndef GAMETICKLER_H
#define GAMETICKLER_H


/*
**	class GameTickler
**
**	add this mixin class to any HIDialog, and the game will continue to play
**	even whilst the dialog's own modal event loop is running. (I hope.)
*/
class GameTickler : public Recurring
{
	virtual void	DoTimer( EventLoopTimerRef /* eltr */ )
						{
						Idle();
						}
protected:
	virtual void	Idle() { if ( gDTSApp ) gDTSApp->Idle(); }
	
public:
					GameTickler()
						{
						InstallTimer( GetCurrentEventLoop(), 0.5, 0.1 );
						}
};


#endif  // GAMETICKLER_H
