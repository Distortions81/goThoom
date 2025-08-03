/*
**	CarbonEvent_mac.h		DTSLib Mac OS X
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

#ifndef CARBONEVENT_MAC_H
#define CARBONEVENT_MAC_H

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "Local_dts.h"

/*
**	class CarbonEvent
**	simple wrapper for a toolbox EventRef, along with an optional EventHandlerCallRef.
**	(so you don't have to keep passing both of those around amongst your event handlers.)
**	Some parts of this code were inspired by PowerPlantX.
*/
class CarbonEvent
{
public:
								// default constructor
							CarbonEvent() :
								mEvent(), mCall()
								{}
								
								// useful constructor, for inside an EventHandlerProc
							CarbonEvent( EventRef inEvt, EventHandlerCallRef inCall = nullptr ) :
								mEvent( inEvt ),
								mCall( inCall )
									{
									__Check( inEvt );
									RetainEvent( inEvt );
									}
								
								// constructor for events that you intend to send or post
							CarbonEvent(
								FourCharCode	inClass,
								UInt32			inKind,
								EventAttributes	inAttr = kEventAttributeNone,
								EventTime		inTime = 0,		// "now"
								CFAllocatorRef	inAlloc = kCFAllocatorDefault );
								
								// copy ctor
							CarbonEvent( const CarbonEvent& inEvt ) :
								mEvent( CopyEvent( inEvt.mEvent ) ),
								mCall( inEvt.mCall )
									{}
								
								// assignment
	CarbonEvent&			operator=( const CarbonEvent& inEvt );
							
								// dtor
							~CarbonEvent()
								{
								if ( mEvent )
									ReleaseEvent( mEvent );
								}
	
		// getters
							operator EventRef() const		{ return mEvent; }
	EventRef				GetEventRef() const				{ return mEvent; }
	FourCharCode			Class() const					{ return GetEventClass( mEvent ); }
	UInt32					Kind() const					{ return GetEventKind( mEvent ); }
	EventTime				Time() const					{ return GetEventTime( mEvent ); }
	
		// event parameter utils
							// note that the arguments to this function are not in the same order
							// as those of the toolbox's GetEventParameter(). I've moved all
							// the can-be-NULL ones to the end, so that they can be optional.
	OSStatus				GetParameter(
								EventParamName		inName,
								EventParamType		inType,
								ByteCount			inSize,
								void *				outData = nullptr,
								ByteCount *			outActSize = nullptr,
								EventParamType *	outActType = nullptr ) const
				{
				// NB. if inType is typeWildCard, then outActType must not be NULL
				OSStatus err = GetEventParameter( mEvent, inName, inType,
									outActType, inSize, outActSize, outData );
#ifdef DEBUG_VERSION
				if ( eventParameterNotFoundErr != err )
					__Check_noErr( err );
#endif
				return err;
				}
	
	OSStatus				SetParameter(
								EventParamName		inName,
								EventParamType		inType,
								ByteCount			inSize,
								const void *		inData ) const
				{
				OSStatus err = SetEventParameter( mEvent, inName, inType, inSize, inData );
				__Check_noErr( err );
				return err;
				}
	
								// post event to a specific event queue
								// (current queue if you don't specify)
	OSStatus				Post(
								EventTargetRef	target,
								EventQueueRef	queueRef = nullptr,
								EventPriority	prio = kEventPriorityStandard );
	
								// send event to a specific target
	OSStatus				Send(
								EventTargetRef	target,
								OptionBits		options = 0 );
	
		// "superclass" invoker, as it were
	OSStatus				CallNextHandler();
	
private:
	EventRef			mEvent;		// the system event itself
	EventHandlerCallRef	mCall;		// the callback reference
};


/*
**	class CarbonEventResponder
**	abstract mixin class for things that want to react to CarbonEvents
**	[This should really be in DTSLib.
**	Come to think of it, we should probably have a DTSLib_v3 that knows about
**	not only CarbonEvents but also HFS+, CoreFoundation, HIViews, etc...]
*/
class CarbonEventResponder
{
public:
							CarbonEventResponder() : mEventHandler() {}
	virtual					~CarbonEventResponder();
	
private:
						// no copying, please
							CarbonEventResponder( const CarbonEventResponder& );
	CarbonEventResponder&	operator=( const CarbonEventResponder& );
	
public:

	// ------ initialization & setup ------
	
								// you must call InitTarget() to set up the event-handler ref
	virtual OSStatus		InitTarget( EventTargetRef inTarget );
	OSStatus				RemoveHandler();
	
	// ------ reacting to events ------
	
								// You MUST override this to receive CarbonEvents. Be sure to
								// call up to superclass for unhandled events.
	virtual OSStatus		HandleEvent( CarbonEvent& ) = 0;
	
	
	// ------ registration & deregistration ------
	
								// add new event-specs to the current handler
	DTSError				AddEventTypes( uint numTypes, const EventTypeSpec * list ) const;
	
								// remove event specs
	DTSError				RemoveEventTypes( uint numTypes, const EventTypeSpec * list ) const;
	
								// single event conveniences for the above
	DTSError				AddEventType( const EventTypeSpec& spec ) const
								{
								return AddEventTypes( 1, &spec );
								}
	
								// remove event specs
	DTSError				RemoveEventType( const EventTypeSpec& spec ) const
								{
								return RemoveEventTypes( 1, &spec );
								}
	
private:

	// ------ internals ------
								// toolbox callback dispatcher
	static OSStatus			DoCarbonEvent( EventHandlerCallRef, EventRef, void * );
	
								// the event-handler ref for this object
	EventHandlerRef		mEventHandler;
};


/*
**	class HICommandResponder
**	abstract base class, derived from CarbonEventResponder.
**	Derive from this (not CER) if your subclass wants to deal with HICommands.
**	Registration for kEventClassCommand events is -automatic-. Don't include those
**	in your own AddEventTypes() list. Ditto for RemoveEventTypes().
**
**	Also, do NOT process those events in your HandleEvent() override; instead,
**	ignore them there. They'll be caught when you pass the incoming event to
**	your superclass's HandleEvent(), and will then be dispatched -back- to you
**	via your ProcessCommand() and UpdateCommandStatus() methods.
*/
class HICommandResponder : public CarbonEventResponder
{
public:
		// what to do with a menu item, during UpdateCommandStatus()
	enum EState
		{
		enable_NO = 0,
		enable_YES,
		enable_LEAVE_ALONE
		};
	
	typedef CarbonEventResponder	SUPER;
	
								// default constructor -- does nothing.
							HICommandResponder();
	
								// automatically deregisters for all kEventClassCommand events
	virtual					~HICommandResponder();

private:
		// no copying
							HICommandResponder( const HICommandResponder& );
	HICommandResponder&		operator=( const HICommandResponder& );

public:
								// automatically registers for kEventClassCommand events
	virtual OSStatus		InitTarget( EventTargetRef target );
	
								// called when receiving a kEventCommandProcess event.
								// Subclasses MUST override this, to do their actual work.
								// It's "safe" for subclasses to invoke this method in -this-
								// class, but not very useful: the superclass method simply
								// returns an event-not-handled result.
	virtual OSStatus		ProcessCommand( const HICommandExtended&, CarbonEvent& ) = 0;
	
								// called when receiving a kEventCommandUpdateStatus event.
								// Subclasses MUST override this, to do their actual work.
								// It's "safe" for subclasses to invoke this method in -this-
								// class, but not very useful: the superclass method simply
								// returns an event-not-handled result.
								// 
								// upon return, if outEnableIt is enable_NO/YES, then the
								// menu item that is the subject of this inquiry will be 
								// disabled or enabled accordingly; otherwise, it'll be left as is.
								// (This happens independently of the overall return value, so
								// that subclass can alter menu status while still returning
								// 'eventNotHandledErr', with all that that implies.)
								//
								// likewise, if the outTitle is non-NULL, then it will be assigned
								// to the underlying menu item, then CFRelease'd.
								//		IMPORTANT: don't use CFSTR() to make these strings!
								//		Those react badly to being released.
								//	(CFSTR's are OK, however, if you CFRetain them first...)
								//
								// It would be better to do the outEnableIt and outTitle gyrations
								// -only- if the subclass returns noErr, but that may require
								// that menus in nibs have their auto-disable flags turned on, 
								// among other things.
								//
	virtual OSStatus		UpdateCommandStatus(
								const HICommandExtended&	inCmd,
								CarbonEvent&				ioEvent,
								EState&						outEnableIt,
								CFStringRef&				outTitle ) const = 0;
	
	// helpful utilities
								// extracts optional kEventParamMenuContext parameter.
								// if not present, returns eventParameterNotFoundErr
	OSStatus				GetMenuContext( const CarbonEvent& inEvt, UInt32& outContext ) const;
	
								// extract keyboard modifiers from a command event
	OSStatus				GetKeyModifiers( const CarbonEvent& inEvt, UInt32& outMods ) const;
	
								// like CheckMenuItem(), but addressed by an HICommand
	static void				CheckCommandMenu( const HICommandExtended& cmd, bool bChecked );
//	static OSStatus			EnableCommandMenu( const HICommandExtended& cmd, bool bEnabled );
//	static OSStatus			SetCommandMenuText( const HICommandExtended& cmd, CFStringRef nm );
	
protected:
								// this takes care of dispatching kEventClassCommands, -only-.
								// all others are kicked upstairs to superclass.
	virtual	OSStatus		HandleEvent( CarbonEvent& );
};

/*
**	HICommandResponder::GetMenuContext()
**
**	trivial utility to get at the menu-context event parameter which is sometimes
**	present in a command event. See <CarbonEvents.h>. If the parameter isn't present,
**	this returns an error.
*/
inline OSStatus
HICommandResponder::GetMenuContext( const CarbonEvent& inEvent, UInt32& outContext ) const
{
	return inEvent.GetParameter( kEventParamMenuContext,
							typeUInt32, sizeof outContext, &outContext );
}


/*
**	HICommandResponder::GetKeyModifiers()
**
**	another trivial utility like the above; this one is for the key-modifiers parameter
**	that [optionally] accompanies a kEventCommandProcess event.
**	Returns error if no such param; in that case, you should assume that no
**	modifier keys were pressed.
*/
inline OSStatus
HICommandResponder::GetKeyModifiers( const CarbonEvent& inEvent, UInt32& outMods ) const
{
	OSStatus result = inEvent.GetParameter( kEventParamKeyModifiers,
							typeUInt32, sizeof outMods, &outMods );
	
//	if ( eventParameterNotFoundErr == result )
//		outMods = 0;	// assume no modifiers
	
	return result;
}



/*
**	class Recurring
**	abstract base class for entities that want to receive regular timer pulses.
**	(These are EventLoopTimers, not EventLoopIdleTimers, so you get tickled all the time,
**	not just when the user is inactive.)
*/
class Recurring
{
public:
						Recurring() : mTimer() {}
	virtual				~Recurring() { RemoveTimer(); }
							
							// see InstallEventLoopTimer() documentation
	void 				InstallTimer(
							EventLoopRef inEventLoop,
							EventTimerInterval inFireDelay,
							EventTimerInterval inInterval );
	
	void				RemoveTimer();
	
	bool				IsTimerInstalled() const { return mTimer != nullptr; }
	
	void				SetNextFireTime( EventTimerInterval inNextFire ) const;
	
	void				Invoke( EventLoopTimerRef eltr )	{ DoTimer( eltr ); }
	
private:
	virtual void		DoTimer( EventLoopTimerRef /* eltr */ ) = 0;
	
	// no copying
						Recurring( const Recurring& );
	Recurring&			operator=( const Recurring& );
	
	EventLoopTimerRef	mTimer;
};


#endif  // CARBONEVENT_MAC_H
