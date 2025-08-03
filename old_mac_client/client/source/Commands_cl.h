/*
**	Commands_cl.h		ClanLord
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

#ifndef COMMANDS_CL_H
#define COMMANDS_CL_H

#include "Public_cl.h"


struct CommandDefinition;

// Utility functions
int 	StartNameComparisonFixed( const char * name1, const char * name2 );
// int 	WithinNameComparisonFixed( const char * name1, const char * name2 );


// A function holding class
namespace ClientCommand
{
	void GetWord( SafeString * inLine, SafeString * outDest );
	bool ResolveInt( const SafeString * input, int * num, bool showError );
	bool ResolveBoolean( const SafeString * word, bool * theBoolean, bool showError );
	
	// return codes for HandleCommand
	enum
		{
		kNotHandled = 0,
		kHandled,
		kDelayed
		};
	
	int	 HandleCommand( const char * cmd );
	
	void HandlePlayerCommand( int cmd_id, SafeString * cmdStr, const CommandDefinition * def );
	void HandlePrefCommand( int cmd_id, SafeString * cmdStr, const CommandDefinition * def );
	void HandleMoveCommand( int cmd_id, SafeString * cmdStr );
#if 0
	int  HandleEquipCommand( SafeString * cmdStr );
	int	 HandleUnequipCommand( int cmd_id );
#endif
	void HandleSelectItemCommand( SafeString * cmdStr );
	void HandleMovieCommand( SafeString * cmdStr );
	
	void HandleLoggedServerCommand( int cmd_id, SafeString * cmdStr );
	
	void HandleHelp( SafeString * cmdStr );
	void HandleHelpCommand( const CommandDefinition * def );
}


// A class for a hierarchical command syntax with help
struct CommandDefinition
{
	const char * 				cdKeyword;
	int							cdID;
	const CommandDefinition *	cdSubCommands;
	const char *				cdHelp;
	
	static int	ResolveCommand( const CommandDefinition ** def, SafeString * inputStr );
	
	// Categories to map subfunctions
	enum {
		CatNone = 0,
		CatPlayer,
		CatPref,
		CatMove,
//		CatEquip,
//		CatUnequip,
		CatSelectItem,
		CatMovie
	};
	
	// Server commands that we log
	enum {
		Bug = 1,
		Thinkto,
		Pray,
#ifdef DEBUG_VERSION
		Use,
		UseItem,
		Message,
#endif
		kMaxServerCmds		// not used
	};
	
	// Individual commands
	enum {
		None = 0,
		Help,
		
		Select = MakeLong( CatPlayer, 1 ),
			SelectNext, SelectPrevious, SelectFirst, SelectLast,
		Label,
		Ignore,
		Block,
		Forget,
		WhoFriend,
		
		Pref = MakeLong( CatPref, 1 ),
			PrefMovement,
				PrefMovementToggle, PrefMovementHold,
			
			PrefMessage,
				PrefMessageFallen,PrefMessageClanning,
			
			PrefLargeWindow, PrefShowNames, PrefBrightColors,PrefTimeStamps, PrefMaxNightPercent,
			PrefSoundVolume, PrefNewLogEveryJoin, PrefNoMovieTextLogs, PrefBardVolume,
		
		Move = MakeLong( CatMove, 1 ),
			MoveWalk, MoveRun,
		
//		Equip = MakeLong( CatEquip, 1 ),
		
//		Unequip = MakeLong( CatUnequip, 1 ),
//			UnequipLeft, UnequipRight,
		
		SelectItem = MakeLong( CatSelectItem, 1 ),
		
		RecordMovie = MakeLong( CatMovie, 1 )
	};
};

#endif	// COMMANDS_CL_H

