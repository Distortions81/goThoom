/*
**	CarbonEvent_mac.cp		DTSLib Mac OS X
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

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "CarbonEvent_mac.h"

#pragma mark === CarbonEvent ===


/*
**	CarbonEvent::CarbonEvent()
**	constructor for completely new events, to be dispatched elsewhere in the system
*/
CarbonEvent::CarbonEvent(
	FourCharCode	evtClass,
	UInt32			evtKind,
	EventAttributes	attrs /* = kEventAttributeNone */,
	EventTime		time /* = 0 */,		// 0 means "now"
	CFAllocatorRef	allocator /* = kCFAllocatorDefault */ )
:
	mEvent( nullptr ),
	mCall( nullptr )
{
	__Verify_noErr( CreateEvent( allocator, evtClass, evtKind, time, attrs, &mEvent ) );
}


/*
**	CarbonEvent::operator=()
**	copy constructor
*/
CarbonEvent&
CarbonEvent::operator=( const CarbonEvent& inEvt )
{
	if ( &inEvt != this )
		{
		if ( mEvent )
			ReleaseEvent( mEvent );
		mEvent = CopyEvent( inEvt.mEvent );
		mCall = inEvt.mCall;
		}
	return *this;
}


/*
**	CarbonEvent::CallNextHandler()
**	trivial wrapper for CallNextEventHandler()
*/
OSStatus
CarbonEvent::CallNextHandler()
{
	OSStatus result = eventNotHandledErr;
	if ( mCall )
		result = CallNextEventHandler( mCall, mEvent );
	return result;
}


/*
**	CarbonEvent::Post()
**	post this event onto a system event queue, to be handled whenever the
**	relevant runloop gets around to it.
**	If you don't supply the 'queue' argument, we use the current event queue, which is NOT
**	necessarily the same as the main application queue. So be careful if you're calling this
**	from a secondary thread.
*/
OSStatus
CarbonEvent::Post(
	EventTargetRef	target,
	EventQueueRef	queue, /* = nullptr */
	EventPriority	prio  /* = kEventPriorityStandard */ )
{
	OSStatus err = noErr;
	if ( target )
		{
		err = SetEventParameter( mEvent, kEventParamPostTarget,
					typeEventTargetRef, sizeof target, &target );
		__Check_noErr( err );
		}
	if ( noErr == err )
		{
		if ( not queue )
			queue = GetCurrentEventQueue();
//		SetEventTime( mEvent, GetCurrentEventTime() );		// ?? need this?
		err = PostEventToQueue( queue, mEvent, prio );
		__Check_noErr( err );
		}
	return err;
}


/*
**	CarbonEvent::Send()
**	send an event directly to a handler. Bypass all queues.
**	'options' are the bits defined in <CarbonEventsCore.h>, i.e.,
**		kEventTargetDontPropagate
**		kEventTargetSendToAllHandlers
**	and so forth (these are in 10.2+ only).
*/
OSStatus
CarbonEvent::Send(
	EventTargetRef	target,
	OptionBits		options /* = kNilOptions */ )
{
//	SetEventTime( mEvent, GetCurrentEventTime() );		// ?? need this?
	OSStatus err = SendEventToEventTargetWithOptions( mEvent, target, options );
	return err;
}

#pragma mark === CarbonEventResponder ===


/*
**	CarbonEventResponder::~CarbonEventResponder()
**	destructor
*/
CarbonEventResponder::~CarbonEventResponder()
{
	(void) RemoveHandler();
}


/*
**	CarbonEventResponder::RemoveHandler()
**	uninstall our handler
**	NB: you have to be a little careful about nesting these.
*/
OSStatus
CarbonEventResponder::RemoveHandler()
{
	OSStatus result = noErr;	// ?? eventInternalErr;
	if ( mEventHandler )
		{
		result = RemoveEventHandler( mEventHandler );
		__Check_noErr( result );
		mEventHandler = nullptr;
		}
	return result;
}


/*
**	CarbonEventResponder::InitTarget()
*/
OSStatus
CarbonEventResponder::InitTarget( EventTargetRef inTarget )
{
	// require that this only be called once.
	// [this class should likely be a virtual base...]
	__Check( nullptr == mEventHandler );
	
	// install empty handler; subclasses can register for additional events
	// by calling AddEventTypes()
	OSStatus err = InstallEventHandler( inTarget, DoCarbonEvent,
						0, nullptr, this, &mEventHandler );
	__Check_noErr( err );
	
	return err;
}


/*
**	CarbonEventResponder::AddEventTypes()
**	listen for additional carbon events
*/
DTSError
CarbonEventResponder::AddEventTypes( uint numTypes, const EventTypeSpec * list ) const
{
	__Check( mEventHandler );
	if ( not mEventHandler )
		return paramErr;
	
	OSStatus err = AddEventTypesToHandler( mEventHandler, numTypes, list );
	__Check_noErr( err );
	return err;
}


/*
**	CarbonEventResponder::RemoveEventTypes()
**	stop listening
*/
DTSError
CarbonEventResponder::RemoveEventTypes( uint num, const EventTypeSpec * list ) const
{
//	__Check( mEventHandler );
	if ( not mEventHandler )
		return paramErr;
	
	OSStatus err = RemoveEventTypesFromHandler( mEventHandler, num, list );
	__Check_noErr( err );
	return err;
}


/*
**	CarbonEventResponder::DoCarbonEvent()			[ static ]
**	dispatch carbon events (coming in from the OS) onward to our class hierarchy
*/
OSStatus
CarbonEventResponder::DoCarbonEvent( EventHandlerCallRef call, EventRef evt, void * ud )
{
	OSStatus result = eventNotHandledErr;
	CarbonEventResponder * responder = static_cast<CarbonEventResponder *>( ud );
	if ( not responder )
		return result;
	
	try
		{
		CarbonEvent event( evt, call );
		result = responder->HandleEvent( event );
		}
	catch ( ... )
		{
		}
	
	return result;
}


/*
**	CarbonEventResponder::HandleEvent()
**	react to carbon events
**	This is just a trivial do-nothing handler; subclasses will have more extensive implementations
*/
OSStatus
CarbonEventResponder::HandleEvent( CarbonEvent& /* evt */ )
{
	return eventNotHandledErr;
}

#pragma mark === HICommandResponder ===


/*
**	kCommandEvents
**	command events understood by HICommandResponder
*/
const EventTypeSpec kCommandEvents[] =
{
	{ kEventClassCommand, kEventProcessCommand },
	{ kEventClassCommand, kEventCommandUpdateStatus }
};


/*
**	HICommandResponder::HICommandResponder()
**	default constructor
*/
HICommandResponder::HICommandResponder()
{
}


/*
**	HICommandResponder::~HICommandResponder()
**
**	destructor. Deregisters for all command events
*/
HICommandResponder::~HICommandResponder()
{
	(void) RemoveEventTypes( GetEventTypeCount(kCommandEvents), kCommandEvents );
}


/*
**	HICommandResponder::InitTarget()
**	"designated initializer"
**	calls superclass for basic setup, then -automatically- registers for all Command events.
*/
OSStatus
HICommandResponder::InitTarget( EventTargetRef target )
{
	OSStatus result = CarbonEventResponder::InitTarget( target );
	if ( noErr == result )
		result = AddEventTypes( GetEventTypeCount(kCommandEvents), kCommandEvents );
	
	return result;
}


/*
**	HICommandResponder::HandleEvent()
**	this routine only handles Command events; all others are passed up to superclass.
**	We extract the HICommand [really, HICommandExtended] event param, then
**	revector the event back to subclass methods
*/
OSStatus
HICommandResponder::HandleEvent( CarbonEvent& event )
{
	OSStatus result = eventNotHandledErr;
	if ( kEventClassCommand == event.Class() )
		{
		UInt32 kind = event.Kind();
		HICommandExtended command;
		OSStatus err = event.GetParameter( kEventParamDirectObject,
							typeHICommand, sizeof command, &command );
		__Check_noErr( err );
		if ( err != noErr )
			return err;
		
		switch ( kind )
			{
				// just do it
			case kEventCommandProcess:
				result = ProcessCommand( command, event );
				break;
			
				// update UI to reflect availability/applicability of a command
				// typically this involves enabling/disabling menu items, and possibly
				// also changing the menu's wording.  Subclasses tell us which of these
				// actions to perform (if any); we do the dirty work.
			case kEventCommandUpdateStatus:
				{
					// assume no change to enable-state or text
				EState enableIt = enable_LEAVE_ALONE;
				CFStringRef itemText = nullptr;
				
					// hey, subclass, what's the story?
				result = UpdateCommandStatus( command, event, enableIt, itemText );
				
					// apply request menu changes
				if ( command.attributes & kHICommandFromMenu )
					{
					MenuRef menu = command.source.menu.menuRef;
					MenuItemIndex item = command.source.menu.menuItemIndex;
					
					if ( enable_NO == enableIt )
						DisableMenuItem( menu, item );
					else
					if ( enable_YES == enableIt )
						EnableMenuItem( menu, item );
					// ... else leave alone.
					
						// don't send me plain CFSTR() strings, cos they dislike being
						// released.  Text from CFCopyLocalizedString() is fine, though.
						// ... Or just call CFRetain() on your CFSTR string.
					if ( itemText )
						{
						err = SetMenuItemTextWithCFString( menu, item, itemText );
						__Check_noErr( err );
						CFRelease( itemText );
						}
					}
				}
				break;
			}
		}
	
	if ( eventNotHandledErr == result )
		result = CarbonEventResponder::HandleEvent( event );
	
	return result;
}


/*
**	HICommandResponder::ProcessCommand()
**
**	empty stub, which subclass MUST override.
*/
OSStatus
HICommandResponder::ProcessCommand(
	const HICommandExtended&	/* inCmd */,
	CarbonEvent&				/* event */ )
{
	return eventNotHandledErr;
}


/*
**	HICommandResponder::UpdateCommandStatus()
**
**	empty stub, which subclasses MUST override.
*/
OSStatus
HICommandResponder::UpdateCommandStatus(
	const HICommandExtended&	/* inCmd */,
	CarbonEvent&				/* event */,
	EState&						/* outState */,
	CFStringRef&				/* outItemName */ ) const
{
	return eventNotHandledErr;
}


/*
**	HICommandResponder::CheckCommandMenu()
**
**	utility to check or uncheck a menu item. Presumably, the menu item is the
**	one associated with the specific HICommand that triggered your UpdateCommandStatus()
**	override method.
*/
void
HICommandResponder::CheckCommandMenu( const HICommandExtended& cmd, bool bChecked )
{
	// if the HICommand really did arise from a menu, we can twiddle it directly
	if ( cmd.attributes & kHICommandFromMenu )
		{
		MenuRef menu = cmd.source.menu.menuRef;
		MenuItemIndex item = cmd.source.menu.menuItemIndex;
		CheckMenuItem( menu, item, bChecked );
		}
	// else, you have a logic error!
}

#pragma mark === Recurring Timers ===


/*
**	TimerCallback()
**	callback vector for Recurring class
**	simply invokes the Idler's periodic action method
*/
static void
TimerCallback( EventLoopTimerRef inTimerRef, void * ud )
{
	Recurring * idler = static_cast<Recurring *>( ud );
	try
		{
		idler->Invoke( inTimerRef );
		}
	catch ( ... )
		{
		}
}


/*
**	Recurring::Install()
**	install an event loop timer on the present object.
**	cancels any previously-installed timer.
*/
void
Recurring::InstallTimer(
	EventLoopRef		inEvtLoop,
	EventTimerInterval	delay,
	EventTimerInterval	repeat )
{
	if ( mTimer )
		RemoveTimer();
	
	__Verify_noErr( InstallEventLoopTimer( inEvtLoop, delay, repeat,
						TimerCallback, this, &mTimer ) );
}


/*
**	Recurring::RemoveTimer()
**	remove the current timer, if there is one
*/
void
Recurring::RemoveTimer()
{
	if ( mTimer )
		{
		__Verify_noErr( RemoveEventLoopTimer( mTimer ) );
		mTimer = nullptr;
		}
}


/*
**	Recurring::SetNextFireTime()
**	schedule the time of the next notification
**	pass kEventDurationForever to suppress them for good.
*/
void
Recurring::SetNextFireTime( EventTimerInterval when ) const
{
	if ( mTimer )
		{
		__Verify_noErr( SetEventLoopTimerNextFireTime( mTimer, when ) );
		}
}

