program jtty_oracle
  use jtty_mod
  use iso_fortran_env, only: int64
  implicit none
  character(len=16) command
  character(len=80) message,decoded
  character(len=512) path,arg
  character(len=34) frames(MAX_FRAMES)
  integer tones(59*MAX_FRAMES),nframes,nsym,nwave,nbytes,unit,i
  integer nhard,nsync
  integer*2,allocatable :: pcm(:)
  real,allocatable :: wave(:)
  complex,allocatable :: cwave(:)
  real f0,xdt,f1,snr,dmin
  logical synced,success
  character(len=2400) blocks
  integer(int64) ids(30)
  real frequencies(30),starts(30)
  logical*1 last(30)
  integer snrs(30),count
  call get_command_argument(1,command)
  call get_command_argument(2,message)
  call get_command_argument(3,path)
  call get_command_argument(4,arg)
  f0=1500.0
  if(len_trim(arg)>0)read(arg,*)f0
  if(trim(command).eq.'encode')then
     call pack_jtty(message,frames,nframes)
     if(nframes.le.0)stop 1
     write(*,'(a)')trim(message)
     write(*,'(i0)')nframes
     do i=1,nframes
        write(*,'(a)')frames(i)
     enddo
     call genjtty(message,tones,nsym)
     write(*,'(*(i1))')tones(1:nsym)
     nwave=384*nsym
     allocate(wave(nwave),cwave(nwave),pcm(nwave+7200))
     call gen_jttywave(tones,nsym,384,2.0,12000.0,f0,cwave,wave,0,nwave)
     pcm=0
     pcm(2401:2400+nwave)=nint(14000.0*wave)
     open(newunit=unit,file=trim(path),access='stream',form='unformatted',status='replace')
     write(unit)pcm
     close(unit)
  else if(trim(command).eq.'decode'.or.trim(command).eq.'decode-many')then
     inquire(file=trim(path),size=nbytes)
     allocate(pcm(nbytes/2))
     open(newunit=unit,file=trim(path),access='stream',form='unformatted',status='old')
     read(unit)pcm
     close(unit)
     if(trim(command).eq.'decode-many')then
        call rjtty_sub(pcm,0,384,200,2800,f0,20.0)
        call rjtty_sub(pcm,size(pcm),384,200,2800,f0,20.0)
        call jtty_get_updates(blocks,ids,frequencies,starts,last,snrs,count)
        if(count.eq.0)stop 2
        do i=1,count
           write(*,'(i0,a,a)')snrs(i),'|',trim(blocks((i-1)*80+1:i*80))
        enddo
        stop
     endif
     synced=.false.;xdt=0.;f1=f0;snr=-99.
     call jtty_decode(pcm,size(pcm),384,f0,20.0,4.6,synced,xdt,f1,snr,decoded,success,nhard,nsync,dmin)
     if(.not.success)stop 2
     write(*,'(a)')trim(decoded)
  else
     stop 3
  endif
end program jtty_oracle
