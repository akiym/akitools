#!perl
use strict;
use warnings;
use Encode;
use Text::Xslate;
use Text::Markdown qw/markdown/;
use Data::Section::Simple qw/get_data_section/;
use File::Zglob;
use Getopt::Long;
use Pod::Usage;

my $slide = shift or pod2usage("Missing filename\n");
my $static_dir = 'static';
if (my $dir = (zglob('**/static'))[0]) {
    $static_dir = $dir;
}
GetOptions(
    'dir=s'  => \$static_dir,
    'output' => \my $output,
    'h|help' => \my $help,
) or pod2usage(2);
pod2usage(1) if $help;

my $content = do {
    open my $fh, '<', $slide or die $!;
    local $/; <$fh>;
};

my @slides = map { markdown($_) } split /----\n/, $content;
my ($title) = $slides[0] =~ m!<h1\s*[\w="]*>(.*?)</h1>!;

my $tx = Text::Xslate->new({
    path => [get_data_section()],
});
my $html = $tx->render('slide.tx', {
    slides     => \@slides,
    title      => $title,
    static_dir => $static_dir,
});

if ($output) {
    (my $filename = $slide) =~ s/\..+$//;
    open my $fh, '>:encoding(utf-8)', "$filename.html" or die $!;
    print {$fh} $html;
} else {
    print encode_utf8($html);
}

__END__

=head1 NAME

slide.pl - Slide generator written in Markdown

=head1 SYNOPSIS

    % slide.pl filename

        --dir=dir   Directory that contains CSS, JavaScript files (default: "static")
        --output

        --help      Show this help

=cut

__DATA__

@@ slide.tx
<!doctype html>
<html>
<head>
    <meta charset="utf-8" />
    <title><: $title :></title>
    <link rel="stylesheet" href="<: $static_dir :>/css/reset.css" type="text/css" />
    <link rel="stylesheet" href="<: $static_dir :>/css/slide.css" type="text/css" />
    <script type="text/javascript" src="<: $static_dir :>/js/jquery-1.6.2.min.js"></script>
    <script type="text/javascript" src="<: $static_dir :>/js/jquery.presentation.js"></script>
</head>
<body>
    <div id="slides">
    : for $slides -> $slide {
        <div class="slide">
<: $slide | mark_raw :>        </div>
    : }
    </div>
    <script>
$(function () {
    $('#slides').presentation();
});
    </script>
</body>
</html>
