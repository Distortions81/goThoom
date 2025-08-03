/*
**	Macros_cl.h		ClanLord
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

#ifndef	MACROS_CL_H
#define MACROS_CL_H


//
//	Interface functions
//
DTSError	InitMacros();
void		KillMacros();

void 		StopMacrosOnKey();

void		DoMacroReplacement( int ch, uint mods );
bool		DoMacroKey( int * ch, uint mods );
bool		DoMacroClick( const DTSPoint * loc, uint mods, int button = 1, uint chord = 1 );
bool		DoMacroClick( const char * name, uint mods );
bool		DoMacroText( const char * sendbuff );
void 		ContinueMacroExecution();
void		InterruptMacro();
void		SetMacroTextWinBuffer( const char * text );

enum
	{
	kKeyClick			= 1024,
	kKeyClick2,
	kKeyClick3,
	kKeyClick4,
	kKeyClick5,
	kKeyClick6,
	kKeyClick7,
	kKeyClick8,
	kKeyClickMax,		// 8 mouse buttons is enough for anyone, right?
	
	kKeyWheelUp			= 2048,
	kKeyWheelDown,
	kKeyWheelLeft,
	kKeyWheelRight
	};

#endif	// MACROS_CL_H

