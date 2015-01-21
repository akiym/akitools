#!/usr/bin/env perl
use strict;
use warnings;
use Digest::MD5 qw/md5_hex/;
use File::Find::Rule;
use Getopt::Long qw/:config posix_default no_ignore_case bundling permute/;
use Text::Diff;

# dirdiff.pl - requires: `cpanm File::Find::Rule Text::Diff`

GetOptions(
    'I|noinfo' => \my $noinfo,
    'c|check'  => \my $check,
) or usage();

my $dir1 = shift // usage();
my $dir2 = shift // usage();

(my $basedir1 = $dir1) =~ s!/$!!;
(my $basedir2 = $dir2) =~ s!/$!!;

# don't forget "quote" to prevent zsh glob expansion
my $name = shift;

my (@files1, @files2);
if (defined $name) {
    @files1 = File::Find::Rule->file()->name($name)->in($dir1);
    @files2 = File::Find::Rule->file()->name($name)->in($dir2);
} else {
    @files1 = File::Find::Rule->file()->in($dir1);
    @files2 = File::Find::Rule->file()->in($dir2);
}

my (@diff_files, @not_found_files);

for my $file1 (@files1) {
    (my $f1 = $file1) =~ s/$basedir1//;
    my $file;
    my $found = 0;
    for my $file2 (@files2) {
        (my $f2 = $file2) =~ s/$basedir2//;
        if ($f1 eq $f2) {
            if ($check) {
                my $diff = check_diff($file1, $file2);
                if ($diff) {
                    push @diff_files, [$file1, $file2];
                }
            } else {
                my $diff = diff($file1, $file2);
                if ($diff) {
                    print $diff;
                    push @diff_files, [$file1, $file2];
                }
            }
            $found = 1;
            last;
        }
    }
    unless ($found) {
        push @not_found_files, $file1;
    }
}

unless ($noinfo) {
    print "\n=====\n";
    print 'dir1: total ' . scalar @files1 . " files\n";
    print 'dir2: total ' . scalar @files2 . " files\n";
    if (@diff_files) {
        print '    ' . $_->[0] . ' ' . $_->[1] . "\n" for @diff_files;
    }
    if (@not_found_files) {
        print "[?] $_\n" for @not_found_files;
    }
}

sub usage {
    die <<"USAGE";
Usage: $0 'dir1' 'dir2'
    dirdiff dir1 dir2
    dirdiff falsified_sourcecode original_sourcecode '*.php'
USAGE
}

sub check_diff {
    my ($file1, $file2) = @_;
    my $src1 = do { open my $fh, '<', $file1 or die $!; local $/; <$fh> };
    my $src2 = do { open my $fh, '<', $file2 or die $!; local $/; <$fh> };
    return md5_hex($src1) ne md5_hex($src2);
}
