/*
**	Dialog_dts.h		dtslib2
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

#ifndef Dialog_dts_h
#define Dialog_dts_h

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "Local_dts.h"
#include "Platform_dts.h"
#include "View_dts.h"


/*
**	Define the DTSDlgView class
*/
class DTSDlgViewPriv;

class DTSDlgView : public DTSView
{
public:
	DTSImplementNoCopy<DTSDlgViewPriv> priv;
	
	// overloaded routines
	virtual bool		KeyStroke( int ch, uint modifiers );
	
	// customizing routines
	virtual void		Init();
	virtual void		Exit();
	virtual void		Hit( int item );
	virtual void		DrawItem( int item, const DTSRect * bounds );
	virtual bool		KeyFilter( int item, int ch, uint mods );
		// KeyFilter returns true if it wants the dialog to ignore the key stroke
	
	// interface
	void		Present( int id );
	void		Dismiss( int item );
	void		SetValue( int item, int value ) const;
	int			GetValue( int item ) const;
	void		SetText( int item, const char * text ) const;
	void		GetText( int item, size_t maxLength, char * text ) const;
	void		SetNumber( int item, int number ) const;
	int			GetNumber( int item ) const;
	void		SetFloat( int item, double number ) const;
	double		GetFloat( int item ) const;
	void		SelectText( int item, int start, int stop ) const;
	void		GetSelect( int * start, int * stop ) const;
	void		GetBoundRect( int item, DTSRect * bounds ) const;
	void		RedrawItem( int item );
	void		CheckRadio( int checked, int min, int max ) const;
	void		Enable( int item ) const;
	void		Disable( int item ) const;
	void		ShowItem( int item ) const;
	void		HideItem( int item ) const;
	void		SetDefaultItem( int item ) const;
	void		SetCancelItem( int item ) const;
	void		SetPopupText( int item, int menuItem, const char * text ) const;
	void		GetPopupText( int item, int menuItem, char * text, size_t maxLen = 256 ) const;
	int			GetNumPopupItems( int item ) const;
	
	// accessors
	void		GetWhere( DTSPoint * where ) const;
	ulong		GetWhen() const;
	uint		GetModifiers() const;
	int			GetItemHit() const;
};


/*
**	Define the DTSDlgList class
**
**	One "feature" of this class is that you must call Init()
**	within the Init method of the DTSDlgView, and call Exit()
**	within the Exit method of the DTSDlgView.
*/
class DTSDlgListPriv;

class DTSDlgList
{
public:
	DTSImplementNoCopy<DTSDlgListPriv> priv;
	
	virtual 	~DTSDlgList();
	
	// interface
	void		Init( const DTSDlgView * dlg, int item );
	void		Exit();
	void		SetText( int item, const char * text ) const;
	void		GetText( int item, char * text, size_t maxlen = 256 ) const;
	void		SetSelect( int item ) const;
	int			GetSelect() const;
	void		Deselect( int item ) const;
	void		Reset();
	bool		WasDoubleClick() const;
	bool		KeyStroke( int ch, uint modifiers );
	DTSError	Remove( int item );
	void		HideDrawing() const;
	void		ShowDrawing() const;
};


/*
**	Generic Dialogs
*/
enum { kGenericOk, kGenericCancel, kGenericYes, kGenericNo };

// put up a generic error dialog box
void	GenericError( const char * format, ... ) PRINTF_LIKE(1, 2);
// put up a generic ok/cancel dialog box
int		GenericOkCancel( const char * format, ... ) PRINTF_LIKE(1, 2);
// put up a generic yes/no/cancel dialog box
int		GenericYesNoCancel( const char * format, ... ) PRINTF_LIKE(1, 2);


#endif  // Dialog_dts_h
