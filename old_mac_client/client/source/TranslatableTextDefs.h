/*
**	TranslatableTextDefs.h		ClanLord
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
**
**
**
**	Text definitions needed for translation of the client.
**
** 	Note: Only translate messages commonly seen by players. 
**	Don't translate rare error conditions or debug output.
**	Charset to use is MacRoman
*/

#ifndef TRANSLATABLE_TEXT_DEFS_H
#define TRANSLATABLE_TEXT_DEFS_H	

#ifndef rez
#define _(x) x		// for now, a do-nothing macro for future use
#endif


// language names as used by Arindal
#define CLT_LANG_ENGLISH	0
#define CLT_LANG_GERMAN		1
#define CLT_LANG_DUTCH		2
#define CLT_LANG_FRENCH		3


#ifdef ARINDAL
	// use English as default (for now!)
    #ifndef CLT_LANG
		#define CLT_LANG CLT_LANG_ENGLISH
		// #define CLT_LANG CLT_LANG_GERMAN
	#endif
	
	#if CLT_LANG == CLT_LANG_ENGLISH
		#include "TextDefs/Arindal_en.h"
	#elif CLT_LANG == CLT_LANG_GERMAN
		#include "TextDefs/Arindal_de.h"
	#elif CLT_LANG == CLT_LANG_DUTCH
		#include "TextDefs/Arindal_nl.h"
	#elif CLT_LANG == CLT_LANG_FRENCH
		#include "TextDefs/Arindal_fr.h"
	#else
		#error "Unsupported Language for Arindal client!"
	#endif
	
	// these should maybe go elsewhere...
	// Strings that vary per-platform but not per-language
	#define TXTCL_APPNAME		"Arindal"
	#define	TXTCL_APP_NAME		"Arindal"
	
#else
	// Definitions used for ClanLord
	
	// use English as default
    	#ifndef CLT_LANG
		#define CLT_LANG CLT_LANG_ENGLISH
	#endif
	
	#if CLT_LANG == CLT_LANG_ENGLISH
		#include "Clanlord_en.h"
	#else
		#error "Only english supported for Clan Lord Client."
	#endif

	// these should maybe go elsewhere...
	// Strings that vary per-platform but not per-language
	#define TXTCL_APPNAME		"ClanLord"
	#define	TXTCL_APP_NAME		"Clan Lord"
	
#endif // ARINDAL

#endif // TRANSLATABLE_TEXT_DEFS_H

